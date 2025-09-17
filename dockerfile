# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o urlshortener ./cmd/api/main.go

# Stage 2: Create a minimal runtime image
FROM alpine:3.20

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/urlshortener .

# Expose the port (default 8080, configurable via environment)
EXPOSE 8080

# Set environment variables (can be overridden at runtime)
ENV PORT=8080
ENV AWS_REGION=us-east-1
ENV AWS_END_POINT=http://localhost:8000
ENV DYNAMODB_TABLE=Urls
ENV SHARD_ID=0

# Run the application
CMD ["./urlshortener"]
