FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git wget unzip

# Copy Go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY main.go ./

# Build the binary
RUN go build -o loader ./main.go

# Download and prepare domains.txt
RUN wget -O top-1m.csv.zip http://s3-us-west-1.amazonaws.com/umbrella-static/top-1m.csv.zip && \
    unzip top-1m.csv.zip && \
    rm top-1m.csv.zip && \
    cut -d',' -f2 top-1m.csv > domains.txt && \
    rm top-1m.csv

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary and domains.txt
COPY --from=builder /app/loader /app/domains.txt ./

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates

# Set default environment variables
ENV DNS_SERVER=coredns
ENV DNS_PORT=5983
ENV METRICS_URL=http://gameserver:8080/metrics
ENV TARGET_QUEUE_SIZE=100
ENV ADJUST_INTERVAL=10s
ENV CHECK_INTERVAL=5s

# Set entrypoint
ENTRYPOINT ["/app/loader"]
