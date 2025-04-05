FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies required for CGO (SQLite)
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o lebentestbot .

FROM alpine:latest

# Install required packages for SQLite and running the bot
RUN apk add --no-cache ca-certificates tzdata sqlite libc6-compat

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/lebentestbot .

# Copy assets directory
COPY --from=builder /app/assets ./assets

# Create data directory
RUN mkdir -p data

# Set environment variables
ENV BOT_TOKEN=""
ENV DEEPSEEK_API_KEY=""
ENV DB_PATH="/app/data/lebentest.db"

# Run the bot
CMD ["./lebentestbot"]