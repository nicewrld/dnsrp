FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

RUN go build -o gameserver .

EXPOSE 8080

CMD ["./gameserver"]