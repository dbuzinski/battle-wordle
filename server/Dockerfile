# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY server/go.mod server/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY server/ .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o battle-wordle-server ./cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache sqlite

# Copy the binary from builder
COPY --from=builder /app/battle-wordle-server .

# Copy word list
COPY --from=builder /app/word_list.txt .

# Copy config files
COPY config/prod.json ./config/

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./battle-wordle-server"] 