# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Set GOTOOLCHAIN to allow automatic toolchain download if needed
ENV GOTOOLCHAIN=auto

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary
COPY --from=builder /app/server .

# Copy static files
COPY static ./static

# Copy install script
COPY install.sh ./install.sh

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

CMD ["./server"]

