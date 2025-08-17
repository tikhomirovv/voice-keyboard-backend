# Build stage
FROM golang:1.24-alpine AS builder

# Install required system packages
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with optimizations for production
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/voice-key \
    -trimpath

# Final stage - using distroless for smaller and more secure image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary
COPY --from=builder /app/voice-key /app/voice-key

# Create non-root user
USER 65532:65532

# Set working directory
WORKDIR /app

# Expose ports
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/app/voice-key", "health:check"] || exit 1

# Command to run
ENTRYPOINT ["/app/voice-key"]
CMD ["app:start"]
