// dns-server-roleplay/webapp/dns_utils.go
package main

import (
	"context"
	"net"

	"github.com/miekg/dns"
)

func getDNSRequest() (*DNSRequest, error) {
	ctx := context.Background()
	data, err := rdb.BRPop(ctx, 0, "dns_queue").Bytes()
	if err != nil {
		return nil, err
	}

	// Deserialize DNS message
	dnsMsg := new(dns.Msg)
	err = dnsMsg.Unpack(data)
	if err != nil {
		return nil, err
	}

	dnsRequest := &DNSRequest{
		Name: dnsMsg.Question[0].Name,
		Type: dnsMsg.Question[0].Qtype,
		Raw:  data,
	}

	return dnsRequest, nil
}

func createDNSResponse(request *dns.Msg, action string) *dns.Msg {
	response := new(dns.Msg)
	response.SetReply(request)

	switch action {
	case "correct":
		// Generate a correct DNS response
		response.Authoritative = true
		response.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   request.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: net.ParseIP("1.2.3.4"),
			},
		}
	case "corrupt":
		// Generate a corrupt DNS response
		response.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   "corrupt." + request.Question[0].Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    300,
				},
				A: net.ParseIP("0.0.0.0"),
			},
		}
	case "delay":
		// Delay the response (handled elsewhere)
	case "nxdomain":
		// Set NXDOMAIN response
		response.Rcode = dns.RcodeNameError
	}

	return response
}
