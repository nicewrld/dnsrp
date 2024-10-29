FROM golang:1.23 AS build-stage

# since we are treating this like an overlay, where we overlay our directory
# onto coredns, we need to build coredns from source.
WORKDIR /app
RUN apt update && apt install -y git
RUN git clone https://github.com/coredns/coredns.git


WORKDIR /app/coredns
# plugin.cfg controls loading of plugins, we need to add our plugin to it
ADD plugin.cfg /app/coredns/plugin.cfg
# add our plugin to the plugin directory
ADD dnsrp.go /app/coredns/plugin/dnsrp/dnsrp.go
ADD setup.go /app/coredns/plugin/dnsrp/setup.go



# build coredns with our plugin
ENV GOFLAGS=-buildvcs=false
RUN go mod vendor
RUN make gen && make

# Final stage
FROM alpine:latest

WORKDIR /root/
COPY --from=build-stage /app/coredns/coredns .
COPY Corefile .

EXPOSE 53 53/udp

ENTRYPOINT ["./coredns", "-conf", "Corefile"]