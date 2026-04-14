# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sp .

# Stage 2: Runtime
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS (needed to download icons)
RUN apk --no-cache add ca-certificates

# Create a non-root user and group
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Copy binary from builder
COPY --from=builder /app/sp .

# Copy static files (HTML, CSS, JS)
COPY --from=builder /app/index.html .

# Create cache directory for icons
RUN mkdir -p icon_cache && \
    chown -R appuser:appgroup /app

# Change to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the app
CMD ["./sp"]
