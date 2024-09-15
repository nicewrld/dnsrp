// dns-server-roleplay/webapp/handlers.go
package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/miekg/dns"
)

var tmpl = template.Must(template.ParseFiles("templates/index.html"))
var leaderboardTmpl = template.Must(template.ParseFiles("templates/leaderboard.html"))

type DNSRequest struct {
	Name string
	Type uint16
	Raw  []byte
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch a DNS request from the queue
	dnsRequest, err := getDNSRequest()
	if err != nil {
		http.Error(w, "No DNS requests available", http.StatusServiceUnavailable)
		return
	}

	// Store the DNS request in session or hidden field
	http.SetCookie(w, &http.Cookie{
		Name:    "dns_request",
		Value:   string(dnsRequest.Raw),
		Expires: time.Now().Add(5 * time.Minute),
	})

	tmpl.Execute(w, dnsRequest)
}

func actionHandler(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	playerID := getPlayerID(r) // Implement session or authentication

	// Retrieve DNS request from cookie
	cookie, err := r.Cookie("dns_request")
	if err != nil {
		http.Error(w, "Session expired", http.StatusBadRequest)
		return
	}
	dnsRequestData := []byte(cookie.Value)

	// Deserialize DNS message
	dnsRequest := new(dns.Msg)
	err = dnsRequest.Unpack(dnsRequestData)
	if err != nil {
		log.Println("Error unpacking DNS request:", err)
		http.Error(w, "Invalid DNS request", http.StatusInternalServerError)
		return
	}

	// Create DNS response based on action
	dnsResponse := createDNSResponse(dnsRequest, action)

	// Serialize DNS response
	responseData, err := dnsResponse.Pack()
	if err != nil {
		log.Println("Error packing DNS response:", err)
		http.Error(w, "Failed to process action", http.StatusInternalServerError)
		return
	}

	// Send response back to CoreDNS
	ctx := context.Background()
	err = rdb.RPush(ctx, "dns_response:"+dnsRequest.Question[0].Name, responseData).Err()
	if err != nil {
		log.Println("Error enqueuing DNS response:", err)
		http.Error(w, "Failed to send response", http.StatusInternalServerError)
		return
	}

	// Update leaderboard
	updateLeaderboard(playerID, action)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	leaderboard := getLeaderboard()
	leaderboardTmpl.Execute(w, leaderboard)
}

func getPlayerID(r *http.Request) string {
	// For simplicity, use IP address as player ID
	return r.RemoteAddr
}
