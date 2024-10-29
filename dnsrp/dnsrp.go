// dnsrp.go
// yo this is where the dns magic happens
// =====================================
//
// this plugin is the secret sauce that lets us:
// - catch dns requests before they get answered
// - ask our game what to do with them
// - do chaotic things to the responses
//
// it's basically a dns request interceptor that
// lets players choose their own adventure

package dnsrp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"bytes"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
)

// the main plugin struct - keeps track of:
// - where to send captured requests
// - how to talk to the game server
// - what to do next if we fail
type DNSRP struct {
	Next          plugin.Handler  // the next plugin to call if we tap out
	GameServerURL string         // where our game server lives
	Client        *http.Client   // for talking to the game server
}

// this is where we intercept dns requests
func (d DNSRP) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	question := r.Question[0]
	log.Infof("dnsrp plugin invoked for query: %s", question.Name)

	// Prepare the DNS request data to send to the game server
	dnsRequest := DNSRequest{
		Name:  question.Name,
		Type:  dns.TypeToString[question.Qtype],
		Class: dns.ClassToString[question.Qclass],
	}

	// Send the request to the game server
	action, err := d.GetActionFromGameServer(dnsRequest)
	log.Infof("Sending DNS request to game server: %s", d.GameServerURL)
	if err != nil {
		log.Errorf("Error posting to game server: %v", err)
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			log.Warningf("Timeout waiting for game server response, proceeding with default action 'correct'")
			action = "correct"
		} else {
			log.Errorf("Error communicating with game server: %v", err)
			// Fallback to next plugin or return SERVFAIL
			return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
		}
	}

	log.Infof("Action received from game server: %s", action)

	// Create a response based on the action
	msg := new(dns.Msg)
	msg.SetReply(r)

	switch action {
	case "correct":
		// Forward the request to the next plugin (e.g., resolve normally)
		return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
	case "corrupt":
		// Return a corrupt response (e.g., wrong IP address)
		rr, _ := dns.NewRR(fmt.Sprintf("%s A 127.0.0.1", question.Name))
		msg.Answer = []dns.RR{rr}
	case "delay":
		// Delay the response
		time.Sleep(5 * time.Second)
		// Then forward the request
		return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
	case "nxdomain":
		// Return NXDOMAIN
		msg.Rcode = dns.RcodeNameError
	default:
		// Unknown action, fallback to next plugin
		return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
	}

	w.WriteMsg(msg)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface
func (d DNSRP) Name() string { return "dnsrp" }

// GetActionFromGameServer communicates with the game server
func (d DNSRP) GetActionFromGameServer(req DNSRequest) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := d.Client.Post(d.GameServerURL+"/dnsrequest", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var gameResponse DNSResponse
	err = json.NewDecoder(resp.Body).Decode(&gameResponse)
	if err != nil {
		return "", err
	}

	return gameResponse.Action, nil
}

// DNSRequest represents the DNS query sent to the game server
type DNSRequest struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class"`
}

// DNSResponse represents the response from the game server
type DNSResponse struct {
	Action string `json:"action"`
}

// Helper function to check for timeout errors
func isTimeoutError(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
