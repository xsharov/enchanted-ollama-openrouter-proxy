FROM golang:latest AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o ollama-proxy


FROM scratch
LABEL author="Mark Nefedov"
LABEL org.opencontainers.image.source="https://github.com/marknefedov/ollama-openrouter-proxy"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/ollama-proxy /ollama-proxy

EXPOSE 11434
ENTRYPOINT ["/ollama-proxy"]