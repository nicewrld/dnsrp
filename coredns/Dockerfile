# dns-server-roleplay/coredns/Dockerfile
FROM golang:1.23 as builder

WORKDIR /go/src/github.com/coredns/coredns
RUN git clone https://github.com/coredns/coredns.git .

COPY plugins/game plugins/game
RUN echo "game:github.com/nicewrld/dnsrp/coredns/plugins/game" >> plugin.cfg

RUN make

FROM alpine:3.14
COPY --from=builder /go/src/github.com/coredns/coredns/coredns /coredns
COPY corefile /corefile
EXPOSE 53 53/udp
ENTRYPOINT ["/coredns", "-conf", "/corefile"]