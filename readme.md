# Ollama Proxy for OpenRouter

## Description
This repository provides a proxy server that emulates [Ollama's REST API](https://github.com/ollama/ollama) but forwards requests to [OpenRouter](https://openrouter.ai/). It uses the [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai) library under the hood, with minimal code changes to keep the Ollama API calls the same. This allows you to use Ollama-compatible tooling and clients, but run your requests on OpenRouter-managed models.
Currently, it is enough for usage with [Jetbrains AI assistant](https://blog.jetbrains.com/ai/2024/11/jetbrains-ai-assistant-2024-3/#more-control-over-your-chat-experience-choose-between-gemini,-openai,-and-local-models). 

## Features
- **Ollama-like API**: The server listens on `8080` and exposes endpoints similar to Ollama (e.g., `/api/chat`, `/api/tags`).
- **Model Listing**: Fetch a list of available models from OpenRouter.
- **Model Details**: Retrieve metadata about a specific model.
- **Streaming Chat**: Forward streaming responses from OpenRouter in a chunked JSON format that is compatible with Ollama’s expectations.

## Usage
You can provide your **OpenRouter** (OpenAI-compatible) API key through an environment variable or a command-line argument:

### 1. Environment Variable

    export OPENAI_API_KEY="your-openrouter-api-key"
    ./ollama-proxy

### 2. Command Line Argument

    ./ollama-proxy "your-openrouter-api-key"

Once running, the proxy listens on port `8080`. You can make requests to `http://localhost:8080` with your Ollama-compatible tooling.

## Installation
1. **Clone the Repository**:

       git clone https://github.com/your-username/ollama-openrouter-proxy.git
       cd ollama-openrouter-proxy

2. **Install Dependencies**:

       go mod tidy

3. **Build**:

       go build -o ollama-proxy
