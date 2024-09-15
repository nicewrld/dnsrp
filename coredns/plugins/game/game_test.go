// dns-server-roleplay/coredns/plugins/game/game_test.go
package game

import (
	"context"
	"testing"

	"github.com/miekg/dns"
)

func TestGameServeDNS(t *testing.T) {
	ctx := context.Background()
	game := New()

	// Create a DNS request message
	msg := new(dns.Msg)
	msg.SetQuestion("example.com.", dns.TypeA)

	// Mock DNS response writer
	rw := &dns.ResponseWriterMock{}

	// Call ServeDNS
	_, err := game.ServeDNS(ctx, rw, msg)
	if err != nil {
		t.Errorf("ServeDNS returned error: %v", err)
	}
}
