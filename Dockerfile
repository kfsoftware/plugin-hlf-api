# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o plugin-hlf-api

# Final stage
FROM alpine:3.21

# Install CA certificates for TLS
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/plugin-hlf-api .

# Create directory for certificates
RUN mkdir -p /app/crypto

# Expose API port
EXPOSE 8080

# Command to run the application
ENTRYPOINT ["./plugin-hlf-api"]
CMD ["serve"] 