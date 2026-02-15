# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o main .

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates wget

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy only the binary from builder
COPY --from=builder /app/main .

# Switch to non-root user
USER appuser

EXPOSE 3000

# Health check using the /health endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:3000/health || exit 1

CMD ["./main"]
