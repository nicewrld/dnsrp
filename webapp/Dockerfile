# dns-server-roleplay/webapp/Dockerfile
FROM golang:1.20 as builder

WORKDIR /app
COPY . .
RUN go build -o webapp

FROM alpine:3.14
COPY --from=builder /app/webapp /webapp
COPY templates/ /templates
EXPOSE 8080
ENTRYPOINT ["/webapp"]