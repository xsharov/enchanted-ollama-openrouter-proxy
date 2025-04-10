package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
)

var modelFilter map[string]struct{}

func loadModelFilter(path string) (map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	filter := make(map[string]struct{})

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			filter[line] = struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return filter, nil
}

func main() {
	r := gin.Default()
	// Load the API key from environment variables or command-line arguments.
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		if len(os.Args) > 1 {
			apiKey = os.Args[1]
		} else {
			slog.Error("OPENAI_API_KEY environment variable or command-line argument not set.")
			return
		}
	}

	provider := NewOpenrouterProvider(apiKey)

	filter, err := loadModelFilter("models-filter")
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("models-filter file not found. Skipping model filtering.")
			modelFilter = make(map[string]struct{})
		} else {
			slog.Error("Error loading models filter", "Error", err)
			return
		}
	} else {
		modelFilter = filter
		slog.Info("Loaded models from filter:")
		for model := range modelFilter {
			slog.Info(" - " + model)
		}
	}

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Ollama is running")
	})
	r.HEAD("/", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/tags", func(c *gin.Context) {
		models, err := provider.GetModels()
		if err != nil {
			slog.Error("Error getting models", "Error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		filter := modelFilter
		// Construct a new array of model objects with extra fields
		newModels := make([]map[string]interface{}, 0, len(models))
		for _, m := range models {
			// Если фильтр пустой, значит пропускаем проверку и берём все модели
			if len(filter) > 0 {
				if _, ok := filter[m.Model]; !ok {
					continue
				}
			}
			newModels = append(newModels, map[string]interface{}{
				"name":        m.Name,
				"model":       m.Model,
				"modified_at": m.ModifiedAt,
				"size":        270898672,
				"digest":      "9077fe9d2ae1a4a41a868836b56b8163731a8fe16621397028c2c76f838c6907",
				"details":     m.Details,
			})
		}

		c.JSON(http.StatusOK, gin.H{"models": newModels})
	})

	r.POST("/api/show", func(c *gin.Context) {
		var request map[string]string
		if err := c.BindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		modelName := request["name"]
		if modelName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Model name is required"})
			return
		}

		details, err := provider.GetModelDetails(modelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, details)
	})

	r.POST("/api/chat", func(c *gin.Context) {
		var request struct {
			Model    string                         `json:"model"`
			Messages []openai.ChatCompletionMessage `json:"messages"`
		}

		// Parse the JSON request
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		fullModelName, err := provider.GetFullModelName(request.Model)
		if err != nil {
			slog.Error("Error getting full model name", "Error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Call ChatStream to get the stream
		stream, err := provider.ChatStream(request.Messages, fullModelName)
		if err != nil {
			slog.Error("Failed to create stream", "Error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer stream.Close() // Ensure stream closure

		// Set headers for streaming response
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Status(http.StatusOK)

		// Stream responses back to the client
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// End of stream
				break
			}
			if err != nil {
				// Handle errors
				slog.Error("Stream error", "Error", err)
				c.Status(http.StatusInternalServerError)
				c.Writer.Write([]byte("Error streaming: " + err.Error() + "\n"))
				c.Writer.Flush()
				return
			}

			// Build JSON response structure
			responseJSON := map[string]interface{}{
				"model":      fullModelName,
				"created_at": time.Now().Format(time.RFC3339),
				"message": map[string]string{
					"role":    "assistant",
					"content": response.Choices[0].Delta.Content,
				},
				"done":              false,
				"total_duration":    0,
				"load_duration":     0,
				"prompt_eval_count": nil, // Replace with actual prompt tokens if available
				"eval_count":        nil, // Replace with actual completion tokens if available
				"eval_duration":     0,
			}

			// Marshal and send the JSON response
			if err := json.NewEncoder(c.Writer).Encode(responseJSON); err != nil {
				slog.Error("Error encoding response", "Error", err)
				c.Status(http.StatusInternalServerError)
				return
			}

			// Flush data to send it immediately
			c.Writer.Flush()
		}

		// Final response indicating the stream has ended
		endResponse := map[string]interface{}{
			"model":      fullModelName,
			"created_at": time.Now().Format(time.RFC3339),
			"message": map[string]string{
				"role":    "assistant",
				"content": "",
			},
			"done":              true,
			"total_duration":    0,
			"load_duration":     0,
			"prompt_eval_count": nil,
			"eval_count":        nil,
			"eval_duration":     0,
		}
		if err := json.NewEncoder(c.Writer).Encode(endResponse); err != nil {
			slog.Error("Error encoding end response", "Error", err)
		}
	})

	r.Run(":11434")
}
