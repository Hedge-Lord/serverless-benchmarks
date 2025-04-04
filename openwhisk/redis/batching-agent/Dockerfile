FROM golang:1.20-alpine AS builder

WORKDIR /app

# Install necessary tools
RUN apk add --no-cache git

# Copy go.mod file
COPY go.mod ./

# Initialize the module and download dependencies
RUN go mod download && go mod tidy

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o redis-batching-agent ./cmd/agent

# Create a minimal image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/redis-batching-agent /app/

# Expose the port
EXPOSE 8080

# Command to run
CMD ["/app/redis-batching-agent", "--redis-host", "${REDIS_HOST:-localhost}", "--redis-port", "${REDIS_PORT:-6379}"] 