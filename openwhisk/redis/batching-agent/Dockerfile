FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git and dependencies
RUN apk add --no-cache git

# Copy the entire source
COPY . .

# Initialize module and ensure go.sum is properly generated
RUN go mod tidy
RUN go mod download

# First try to get the specific package to ensure it's downloaded properly
RUN go get github.com/serverless-benchmarks/redis-batching-agent/cmd/agent
RUN go get github.com/redis/go-redis/v9
RUN go get github.com/valyala/fasthttp
RUN go get golang.org/x/net/context

# Force vendoring of dependencies to ensure they are all accessible
RUN go mod vendor

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /redis-batching-agent ./cmd/agent

# Use a minimal alpine image for the final container
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /redis-batching-agent /app/redis-batching-agent

# Expose the default port
EXPOSE 8080

# Command to run
CMD ["/app/redis-batching-agent", "--redis-host", "${REDIS_HOST:-localhost}", "--redis-port", "${REDIS_PORT:-6379}"] 