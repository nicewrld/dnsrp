FROM golang:1.21-alpine

WORKDIR /app

# Install build dependencies for SQLite
RUN apk add --no-cache gcc musl-dev

COPY go.mod ./

RUN go mod download

COPY . .

# Build with CGO enabled
RUN CGO_ENABLED=1 go build -o gameserver .

EXPOSE 8080

CMD ["./gameserver"]
