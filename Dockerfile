# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o zyxel-to-mqtt .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and create non-root user
RUN apk --no-cache add ca-certificates && \
    adduser -D -H -u 10001 zyxel

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/zyxel-to-mqtt /app/zyxel-to-mqtt

USER zyxel

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["/app/zyxel-to-mqtt"]
