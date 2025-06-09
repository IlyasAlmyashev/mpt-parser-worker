FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/parser-worker cmd/main.go

# Final stage
FROM alpine:3.19

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/parser-worker .
COPY --from=builder /app/.env .

# Run as non-root user
RUN adduser -D appuser
USER appuser

EXPOSE 8081

ENTRYPOINT ["./parser-worker"]
CMD ["--only=kaspi"]