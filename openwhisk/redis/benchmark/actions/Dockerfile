FROM golang:1.20-alpine AS builder

WORKDIR /app

# Install necessary packages
RUN apk add --no-cache git

# Copy go.mod and go.sum
COPY go.mod ./
RUN go mod download

# Copy source code
COPY redis_benchmark.go ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o redis-benchmark redis_benchmark.go

# Runtime image
FROM openwhisk/dockerskeleton

# Copy binary from builder
COPY --from=builder /app/redis-benchmark /action/exec

# Set permissions
RUN chmod 755 /action/exec 