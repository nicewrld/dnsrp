// dns-server-roleplay/coredns/plugins/game/game.go
package game

import (
	"context"
	"log"

	"github.com/coredns/coredns/plugin"
	"github.com/go-redis/redis/v8"
	"github.com/miekg/dns"
)

type Game struct {
	Next plugin.Handler
	Rdb  *redis.Client
}

func New() *Game {
	return &Game{
		Rdb: redis.NewClient(&redis.Options{
			Addr: "redis:6379",
		}),
	}
}

func (g *Game) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// Serialize DNS message
	data, err := r.Pack()
	if err != nil {
		log.Println("Error packing DNS message:", err)
		return dns.RcodeServerFailure, err
	}

	// Enqueue the DNS request
	err = g.Rdb.LPush(ctx, "dns_queue", data).Err()
	if err != nil {
		log.Println("Error enqueueing DNS request:", err)
		return dns.RcodeServerFailure, err
	}

	// Wait for response from web app
	var responseData []byte
	for {
		responseData, err = g.Rdb.BLPop(ctx, 0, "dns_response:"+r.Question[0].Name).Bytes()
		if err != nil {
			log.Println("Error dequeuing DNS response:", err)
			continue
		}
		break
	}

	// Unpack DNS response
	response := new(dns.Msg)
	err = response.Unpack(responseData)
	if err != nil {
		log.Println("Error unpacking DNS response:", err)
		return dns.RcodeServerFailure, err
	}

	// Write response back to client
	err = w.WriteMsg(response)
	if err != nil {
		log.Println("Error writing DNS response:", err)
		return dns.RcodeServerFailure, err
	}

	return dns.RcodeSuccess, nil
}

func (g *Game) Name() string { return "game" }
