FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./
# RUN go mod download

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
CMD ["/app/redis-batching-agent"] 