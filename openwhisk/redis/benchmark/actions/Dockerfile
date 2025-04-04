FROM golang:1.20-alpine AS builder

WORKDIR /app

# Install necessary packages
RUN apk add --no-cache git

# Copy go.mod initially 
COPY go.mod ./

# Initialize module and ensure go.sum is properly generated
RUN go mod tidy
RUN go mod download

# Explicitly get the Redis package
RUN go get github.com/redis/go-redis/v9

# Copy source code
COPY redis_benchmark.go ./

# Build the binary - explicitly disable vendor mode
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -o redis-benchmark redis_benchmark.go

# Runtime image
FROM openwhisk/dockerskeleton

# Copy binary from builder
COPY --from=builder /app/redis-benchmark /action/exec

# Set permissions
RUN chmod 755 /action/exec 