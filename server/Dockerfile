# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app/server

# Copy go mod and sum files
COPY server/go.mod server/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY server/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o battle-wordle-server

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/server/battle-wordle-server .

# Copy word list
COPY --from=builder /app/server/word_list.txt .

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./battle-wordle-server"] 