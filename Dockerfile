# Build stage
FROM golang:1.22-alpine AS builder

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

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/voice-key

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy the binary and config
COPY --from=builder /app/voice-key .
COPY --from=builder /app/config.yml .

# Command to run
ENTRYPOINT ["/app/voice-key"]
CMD ["app:start"]
