FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod ./

# Download dependencies
RUN go mod download


COPY . .

RUN go build -o webinterface .

EXPOSE 8081

CMD ["./webinterface"]