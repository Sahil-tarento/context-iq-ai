# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install system dependencies if any
RUN apk add --no-cache git

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked executable
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /contextiq ./cmd/contextiq/main.go

# Production stage
FROM alpine:3.19

# Add certificates for SSL connection to providers
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /contextiq /app/contextiq

# Default daemon ports
EXPOSE 8080 50051

ENTRYPOINT ["/app/contextiq"]
