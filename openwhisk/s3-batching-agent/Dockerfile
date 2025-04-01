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
RUN go get github.com/serverless-benchmarks/openwhisk/batching-agent/cmd/agent
RUN go get github.com/aws/aws-sdk-go-v2/config
RUN go get github.com/aws/aws-sdk-go-v2/service/s3
RUN go get github.com/gorilla/mux

# Force vendoring of dependencies to ensure they are all accessible
RUN go mod vendor

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