// setup.go

package dnsrp

import (
	"net/http"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() {
	plugin.Register("dnsrp", setup)
}

func setup(c *caddy.Controller) error {
	dnsrp := &DNSRP{
		Client: &http.Client{
			Timeout: 35 * time.Second, // Set to slightly more than 30 seconds
		},
	}

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) != 1 {
			return plugin.Error("dnsrp", c.ArgErr())
		}
		dnsrp.GameServerURL = args[0]
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		dnsrp.Next = next
		return dnsrp
	})

	return nil
}
