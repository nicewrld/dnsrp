# Stage 1: Build the frontend assets
FROM node:18-alpine AS frontend-builder

# Set the working directory
WORKDIR /app

# Copy the frontend package.json and package-lock.json (if present)
COPY package*.json ./

# Install frontend dependencies
RUN npm install

# Copy the rest of the frontend source code
COPY . ./

# Build the frontend assets
RUN npm run build

# Stage 2: Build the Go backend
FROM golang:1.23-alpine AS backend-builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
COPY go.mod ./
RUN go mod download

# Copy the rest of the backend source code
COPY . ./

# Copy the built frontend assets from the previous stage
COPY --from=frontend-builder /app/public ./public

# Build the Go application
RUN go build -o webinterface .

# Stage 3: Create the final image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the Go application binary from the backend-builder stage
COPY --from=backend-builder /app/webinterface ./

# Copy the public directory containing frontend assets
COPY --from=backend-builder /app/public ./public

# Expose the application port
EXPOSE 8081

# Run the application
CMD ["./webinterface"]
