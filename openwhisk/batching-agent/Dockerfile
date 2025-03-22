FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod ./

# Install any dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /batching-agent ./cmd/agent

# Use a minimal alpine image for the final container
FROM alpine:3.16

WORKDIR /

# Copy the binary from the builder stage
COPY --from=builder /batching-agent /batching-agent

# Expose the default port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/batching-agent"] 