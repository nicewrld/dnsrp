package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestDNSRequestHandler tests the dnsRequestHandler function
func TestDNSRequestHandler(t *testing.T) {
	// Initialize necessary variables and state
	dnsRequests = make(map[string]*DNSRequest)
	pendingActions = sync.Map{}
	dnsRequestChan = make(chan *DNSRequest, MaxDNSQueueSize)

	// Create a sample DNSRequest
	reqBody := DNSRequest{
		Name:  "example.com",
		Type:  "A",
		Class: "IN",
	}
	body, _ := json.Marshal(reqBody)

	// Create a request
	req, err := http.NewRequest("POST", "/dnsrequest", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(dnsRequestHandler)

	// Call the handler
	go func() {
		// Simulate player action after a delay
		time.Sleep(1 * time.Second)
		pendingActions.Range(func(key, value interface{}) bool {
			if actionChan, ok := value.(chan string); ok {
				actionChan <- "correct"
			}
			return false
		})
	}()

	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("dnsRequestHandler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body
	var resp DNSResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Errorf("Failed to parse response body: %v", err)
	}

	if resp.Action != "correct" {
		t.Errorf("Expected action 'correct', got '%s'", resp.Action)
	}
}

// TestRegisterHandler tests the registerHandler function
func TestRegisterHandler(t *testing.T) {
	players = make(map[string]*Player)

	// Create a request with nickname
	req, err := http.NewRequest("GET", "/register?nickname=TestPlayer", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(registerHandler)

	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("registerHandler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body for player ID
	playerID := rr.Body.String()
	if playerID == "" {
		t.Errorf("Expected a player ID, got empty string")
	}

	// Verify the player is registered
	playersMu.RLock()
	player, exists := players[playerID]
	playersMu.RUnlock()
	if !exists {
		t.Errorf("Player ID %s was not registered", playerID)
	}

	if player.Nickname != "TestPlayer" {
		t.Errorf("Expected nickname 'TestPlayer', got '%s'", player.Nickname)
	}
}

// Additional test functions for other handlers and functionalities can be added similarly
