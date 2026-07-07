# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off
# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o webhook-buffer .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/webhook-buffer .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./webhook-buffer"]
