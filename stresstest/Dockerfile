# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git wget unzip

# Copy Go modules files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY stresstest.go ./

# Build the binary
RUN go build -o stresstest ./stresstest.go

# Download and prepare domains.txt
RUN wget -O top-1m.csv.zip http://s3-us-west-1.amazonaws.com/umbrella-static/top-1m.csv.zip && \
    unzip top-1m.csv.zip && \
    rm top-1m.csv.zip && \
    cut -d',' -f2 top-1m.csv > domains.txt && \
    rm top-1m.csv

# Final image
FROM alpine:latest

WORKDIR /app

# Copy the binary and domains.txt
COPY --from=builder /app/stresstest /app/domains.txt ./

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates

# Set default environment variables
ENV NUM_PLAYERS=500
ENV MAX_WORKERS=100
ENV NUM_THREADS=100
ENV DNS_SERVER=coredns
ENV DNS_PORT=5983
ENV WEB_INTERFACE_HOST=webinterface:8081
ENV STARTUP_DELAY=30

# Set entrypoint
ENTRYPOINT ["/app/stresstest"]