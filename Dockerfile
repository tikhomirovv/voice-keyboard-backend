# Build stage
FROM golang:1.24-alpine AS builder

# Install required system packages
RUN apk add --no-cache git make dumb-init

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

# Copy views directory for templates
COPY --from=builder /app/internal/views /app/internal/views

# Copy dumb-init from builder
COPY --from=builder /usr/bin/dumb-init /usr/bin/dumb-init

# Create non-root user
USER 65532:65532

# Set working directory
WORKDIR /app

# Expose ports
EXPOSE 8080

# Command to run with dumb-init as PID 1
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/app/voice-key", "app:start"]
