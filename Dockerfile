# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (including OpenSSL for cert generation)
RUN apk --no-cache add ca-certificates tzdata openssl

# Copy binary from builder
COPY --from=builder /app/main .

# Copy web assets (templates, static files)
COPY --from=builder /app/web ./web

# Copy internal SQL schemas (for migrations)
COPY --from=builder /app/internal/models/*.sql ./internal/models/

# Copy entrypoint script
COPY --from=builder /app/entrypoint.sh .
RUN chmod +x entrypoint.sh

# Create directories
RUN mkdir -p ./web/uploads ./certs

# Expose port
EXPOSE 8080

# Use entrypoint script
ENTRYPOINT ["./entrypoint.sh"]
