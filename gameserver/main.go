// gameserver/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/nicewrld/gameserver/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Metrics
	dnsRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gameserver_dns_requests_total",
		Help: "Total number of DNS requests received",
	})
	dnsRequestQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gameserver_dns_request_queue_size",
		Help: "Current size of the DNS request queue",
	})
	dnsRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gameserver_dns_request_duration_seconds",
		Help:    "DNS request latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"action"})
	playerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gameserver_player_count",
		Help: "Current number of registered players",
	})
	playerActionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gameserver_player_actions_total",
		Help: "Total number of actions by type",
	}, []string{"action"})
)

// DNSRequest represents a DNS query received by the gameserver
type DNSRequest struct {
	RequestID string    `json:"request_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Class     string    `json:"class"`
	Assigned  bool      `json:"assigned"`
	Timestamp time.Time `json:"timestamp"`
}

// **Define DNSResponse type**
type DNSResponse struct {
	Action string `json:"action"`
}

// Player represents a player in the game
type Player struct {
	ID                string
	Nickname          string
	PurePoints        float64
	EvilPoints        float64
	AssignedRequestID string // Field to track assigned DNS request
}

var (
	dnsRequests    = make(map[string]*DNSRequest) // Map of request ID to DNSRequest
	players        = make(map[string]*Player)     // Map of player ID to Player
	pendingActions sync.Map                       // Map of request ID to action channel

	dnsRequestsMu sync.RWMutex // Read-Write Mutex for dnsRequests
	playersMu     sync.RWMutex // Read-Write Mutex for players

	dnsRequestChan = make(chan *DNSRequest, 10000) // Buffered channel for DNS requests
)

const (
	MaxDNSQueueSize = 10000 // Maximum number of DNS requests in the queue
)

// dnsRequestHandler handles incoming DNS requests from CoreDNS
func dnsRequestHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	dnsRequestsTotal.Inc()
	var dnsReq DNSRequest
	err := json.NewDecoder(r.Body).Decode(&dnsReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a unique RequestID and mark as unassigned
	dnsReq.RequestID = generateRequestID()
	dnsReq.Assigned = false
	dnsReq.Timestamp = time.Now()

	// Create a channel to receive the action
	actionChan := make(chan string, 1) // Buffered to prevent blocking

	// Add the DNS request to the map
	dnsRequestsMu.Lock()
	dnsRequests[dnsReq.RequestID] = &dnsReq
	dnsRequestsMu.Unlock()

	// Store the action channel for later use
	pendingActions.Store(dnsReq.RequestID, actionChan)

	// Enqueue the DNS request
	select {
	case dnsRequestChan <- &dnsReq:
		dnsQueueSize := len(dnsRequestChan)
		dnsRequestQueueSize.Set(float64(dnsQueueSize))
		log.Printf("[RequestID: %s] Received DNS request: %v. Queue size: %d", dnsReq.RequestID, dnsReq, dnsQueueSize)
	default:
		// Queue is full
		log.Printf("[RequestID: %s] DNS request queue is full. Rejecting request: %v", dnsReq.RequestID, dnsReq)
		http.Error(w, "Server busy. Try again later.", http.StatusServiceUnavailable)
		// Clean up
		dnsRequestsMu.Lock()
		delete(dnsRequests, dnsReq.RequestID)
		dnsRequestsMu.Unlock()
		pendingActions.Delete(dnsReq.RequestID)
		return
	}

	// Wait for the player's action or timeout
	var action string
	select {
	case action = <-actionChan:
		// Player responded
	case <-time.After(30 * time.Second):
		// Timeout
		action = "correct" // Default action
	}

	// Send the action back to the DNS plugin
	dnsResp := DNSResponse{Action: action}
	json.NewEncoder(w).Encode(dnsResp)
	
	// Record request duration with action label
	dnsRequestLatency.With(prometheus.Labels{
		"action": action,
	}).Observe(time.Since(start).Seconds())

	// Clean up
	dnsRequestsMu.Lock()
	delete(dnsRequests, dnsReq.RequestID)
	dnsRequestsMu.Unlock()
	pendingActions.Delete(dnsReq.RequestID)
}

// assignDNSRequestHandler assigns DNS requests to players
func assignDNSRequestHandler(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "Missing player_id", http.StatusBadRequest)
		return
	}

	playersMu.Lock()
	player, exists := players[playerID]
	if !exists {
		playersMu.Unlock()
		http.Error(w, "Invalid player_id", http.StatusBadRequest)
		return
	}

	// Check if the player already has an assigned request
	if player.AssignedRequestID != "" {
		dnsRequestsMu.RLock()
		dnsReq, exists := dnsRequests[player.AssignedRequestID]
		dnsRequestsMu.RUnlock()
		if exists && dnsReq.Assigned {
			log.Printf("[PlayerID: %s] Already assigned request %s", playerID, dnsReq.RequestID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(dnsReq)
			playersMu.Unlock()
			return
		}
		// If the assigned request is no longer valid, clear it
		player.AssignedRequestID = ""
	}

	playersMu.Unlock()

	// Assign a new request from the queue
	select {
	case dnsReq := <-dnsRequestChan:
		dnsQueueSize := len(dnsRequestChan)

		// Assign the request to the player
		playersMu.Lock()
		player, exists := players[playerID]
		if !exists {
			playersMu.Unlock()
			// Return the DNS request back to the queue
			dnsRequestChan <- dnsReq
			http.Error(w, "Invalid player_id", http.StatusBadRequest)
			return
		}
		dnsReq.Assigned = true
		player.AssignedRequestID = dnsReq.RequestID
		log.Printf("[PlayerID: %s] Assigned request %s. Queue size after dequeue: %d", playerID, dnsReq.RequestID, dnsQueueSize)
		playersMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dnsReq)
	default:
		// No DNS requests available
		log.Printf("[PlayerID: %s] No DNS requests available", playerID)
		http.Error(w, "No DNS requests available", http.StatusNoContent)
		return
	}
}

// generateRequestID creates a unique RequestID based on the current timestamp
func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

// submitActionHandler handles the actions submitted by players
func submitActionHandler(w http.ResponseWriter, r *http.Request) {
	var actionReq struct {
		PlayerID  string `json:"player_id"`
		RequestID string `json:"request_id"`
		Action    string `json:"action"`
	}
	err := json.NewDecoder(r.Body).Decode(&actionReq)
	if err != nil {
		log.Printf("Failed to decode action request: %v", err)
		http.Error(w, "Invalid request data.", http.StatusBadRequest)
		return
	}

	// Validate player
	playersMu.RLock()
	player, exists := players[actionReq.PlayerID]
	playersMu.RUnlock()
	if !exists {
		log.Printf("Invalid player ID: %s", actionReq.PlayerID)
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}

	// Validate assigned request
	playersMu.RLock()
	assignedRequestID := player.AssignedRequestID
	playersMu.RUnlock()
	if assignedRequestID != actionReq.RequestID {
		log.Printf("Player %s assigned request %s does not match submitted request %s", actionReq.PlayerID, assignedRequestID, actionReq.RequestID)
		http.Error(w, "Invalid request_id for this player", http.StatusBadRequest)
		return
	}

	// Validate DNS request
	dnsRequestsMu.RLock()
	dnsReq, exists := dnsRequests[actionReq.RequestID]
	dnsRequestsMu.RUnlock()
	if !exists || !dnsReq.Assigned {
		log.Printf("Invalid or unassigned DNS request: %s", actionReq.RequestID)
		http.Error(w, "Invalid request or player", http.StatusBadRequest)
		return
	}

	// Update player's score based on the action
	playersMu.Lock()
	switch actionReq.Action {
	case "correct":
		player.PurePoints += 1
		playerActionCounter.With(prometheus.Labels{"action": "correct"}).Inc()
	case "corrupt", "delay", "nxdomain":
		player.EvilPoints += 1
		playerActionCounter.With(prometheus.Labels{"action": actionReq.Action}).Inc()
	default:
		playersMu.Unlock()
		log.Printf("Invalid action submitted by player %s: %s", actionReq.PlayerID, actionReq.Action)
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	// Clear the player's assigned request
	player.AssignedRequestID = ""
	playersMu.Unlock()
	log.Printf("Cleared assigned request for player %s", actionReq.PlayerID)

	// Notify the DNS request handler
	value, ok := pendingActions.Load(actionReq.RequestID)
	if ok {
		actionChan := value.(chan string)
		actionChan <- actionReq.Action
	} else {
		log.Printf("Action channel not found for request %s", actionReq.RequestID)
	}

	log.Printf("Player %s submitted action '%s' for request %s", actionReq.PlayerID, actionReq.Action, actionReq.RequestID)

	w.WriteHeader(http.StatusOK)
}

// leaderboardHandler returns the current leaderboard
func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	type LeaderboardEntry struct {
		PlayerID     string  `json:"player_id"`
		Nickname     string  `json:"nickname"`
		PurePoints   float64 `json:"pure_points"`
		EvilPoints   float64 `json:"evil_points"`
		NetAlignment float64 `json:"net_alignment"`
	}

	playersMu.RLock()
	defer playersMu.RUnlock()

	var leaderboard []LeaderboardEntry
	for _, player := range players {
		leaderboard = append(leaderboard, LeaderboardEntry{
			PlayerID:     player.ID,
			Nickname:     player.Nickname,
			PurePoints:   player.PurePoints,
			EvilPoints:   player.EvilPoints,
			NetAlignment: player.PurePoints - player.EvilPoints,
		})
	}

	// Sort the leaderboard by NetAlignment in descending order
	sort.Slice(leaderboard, func(i, j int) bool {
		return leaderboard[i].NetAlignment > leaderboard[j].NetAlignment
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}

// generatePlayerID creates a unique PlayerID based on the current timestamp
func generatePlayerID() string {
	return fmt.Sprintf("player-%d", time.Now().UnixNano())
}

// registerHandler handles player registration
func registerHandler(w http.ResponseWriter, r *http.Request) {
	nickname := r.URL.Query().Get("nickname")
	if nickname == "" {
		http.Error(w, "Nickname is required", http.StatusBadRequest)
		return
	}

	playerID := generatePlayerID()
	
	// Create new player
	player := &Player{
		ID:         playerID,
		Nickname:   nickname,
		PurePoints: 0,
		EvilPoints: 0,
	}

	// Store in memory
	playersMu.Lock()
	players[playerID] = player
	playerCount.Set(float64(len(players)))
	playersMu.Unlock()

	// Persist to database asynchronously
	go func() {
		if err := db.CreatePlayer(playerID, nickname); err != nil {
			log.Printf("Warning: Failed to persist player %s to database: %v", playerID, err)
		}
	}()

	log.Printf("Registered player: %s (%s)", nickname, playerID)
	w.Write([]byte(playerID))
}

// cleanupExpiredRequests periodically cleans up expired DNS requests
func cleanupExpiredRequests() {
	for {
		time.Sleep(1 * time.Minute)
		dnsRequestsMu.Lock()
		now := time.Now()
		for reqID, dnsReq := range dnsRequests {
			if now.Sub(dnsReq.Timestamp) > 5*time.Minute {
				delete(dnsRequests, reqID)
				log.Printf("Expired DNS request %s cleaned up", reqID)
			}
		}
		dnsRequestsMu.Unlock()
	}
}

// syncPlayersToDatabase periodically syncs the in-memory player state to SQLite
func syncPlayersToDatabase() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		playersMu.RLock()
		for _, player := range players {
			err := db.UpdatePlayerPoints(player.ID, player.PurePoints, player.EvilPoints)
			if err != nil {
				log.Printf("Error syncing player %s to database: %v", player.ID, err)
			}
		}
		playersMu.RUnlock()
		log.Printf("Synced %d players to database", len(players))
	}
}

func main() {
	// Initialize database
	if err := db.Initialize("/litefs/gameserver.db"); err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
	}

	// Load existing players from database
	dbPlayers, err := db.GetLeaderboard()
	if err != nil {
		log.Printf("Warning: Failed to load players from database: %v", err)
	} else {
		playersMu.Lock()
		for _, p := range dbPlayers {
			players[p.ID] = &Player{
				ID:         p.ID,
				Nickname:   p.Nickname,
				PurePoints: p.PurePoints,
				EvilPoints: p.EvilPoints,
			}
		}
		playersMu.Unlock()
		log.Printf("Loaded %d players from database", len(dbPlayers))
	}

	// Start periodic database sync
	go syncPlayersToDatabase()

	// Handle graceful shutdown
	mux := http.NewServeMux()
	
	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())
	
	mux.HandleFunc("/dnsrequest", dnsRequestHandler)
	mux.HandleFunc("/submitaction", submitActionHandler)
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/assign", assignDNSRequestHandler)
	mux.HandleFunc("/leaderboard", leaderboardHandler)

	// Start the DNS request cleanup goroutine
	go cleanupExpiredRequests()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,  // Maximum duration for reading the entire request, including the body
		WriteTimeout: 35 * time.Second, // Maximum duration before timing out writes of the response
		IdleTimeout:  60 * time.Second, // Maximum amount of time to wait for the next request when keep-alives are enabled
	}

	// Channel to listen for errors
	serverErrors := make(chan error, 1)

	// Start the server
	go func() {
		log.Println("Game server running on port 8080")
		serverErrors <- server.ListenAndServe()
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Could not start server: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v. Shutting down...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}

		close(dnsRequestChan) // Close the DNS request channel to stop workers
	}
}
