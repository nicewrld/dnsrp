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
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/nicewrld/gameserver/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//////////////////////////////////////////
// Constants
//////////////////////////////////////////

const (
	// MaxDNSQueueSize defines the maximum number of DNS requests allowed in the queue.
	MaxDNSQueueSize = 10000

	// MinimumRemainingTime defines the minimum time a DNS request must have before timing out to be assigned to a player.
	MinimumRemainingTime = 15 * time.Second
)

//////////////////////////////////////////
// Prometheus Metrics
//////////////////////////////////////////

var (
	// dnsRequestsTotal tracks the total number of DNS requests processed.
	// Useful for capacity planning, traffic analysis, and monitoring growth.
	dnsRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gameserver_dns_requests_total",
		Help: "Total number of DNS requests received since server start",
	})

	// dnsRequestLatency measures the distribution of DNS request processing times.
	// Essential for SLA monitoring and performance optimization.
	dnsRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gameserver_dns_request_duration_seconds",
		Help:    "Time taken to process DNS requests by action type",
		Buckets: prometheus.DefBuckets, // 0.005 to 10 seconds
	}, []string{"action"})

	// playerCount tracks the current number of active players.
	// Useful for capacity planning and engagement monitoring.
	playerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gameserver_player_count",
		Help: "Current number of registered players in the game",
	})

	// playerActionCounter records the distribution of actions chosen by players.
	// Useful for game balance analysis and cheat detection.
	playerActionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gameserver_player_actions_total",
		Help: "Distribution of actions chosen by players",
	}, []string{"action"})

	// pendingDNSRequests monitors the number of DNS requests waiting to be assigned.
	pendingDNSRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gameserver_pending_dns_requests",
		Help: "Current number of DNS requests waiting to be assigned to players",
	})
)

//////////////////////////////////////////
// Core Data Structures
//////////////////////////////////////////

// DNSRequest represents an incoming DNS query from CoreDNS.
type DNSRequest struct {
	RequestID string    `json:"request_id"` // Unique identifier for tracking
	Name      string    `json:"name"`       // Queried domain name
	Type      string    `json:"type"`       // Query type (e.g., A, AAAA)
	Class     string    `json:"class"`      // Query class (usually IN)
	Assigned  bool      `json:"assigned"`   // Indicates if a player has been assigned to handle this request
	Timestamp time.Time `json:"timestamp"`  // Time when the request was received
	TimedOut  bool      // Indicates if the request has timed out
}

// DNSResponse specifies the action to take on a DNS request.
type DNSResponse struct {
	Action string `json:"action"` // Possible actions: correct, corrupt, delay, nxdomain
}

// Player maintains the state and score of a game player.
type Player struct {
	ID                string  // Unique player identifier
	Nickname          string  // Display name of the player
	PurePoints        float64 // Points accumulated from correct responses
	EvilPoints        float64 // Points accumulated from manipulated responses
	PureDelta         float64 // Pending pure point changes to be synced to the database
	EvilDelta         float64 // Pending evil point changes to be synced to the database
	AssignedRequestID string  // ID of the current DNS request assigned to the player
}

//////////////////////////////////////////
// Global Variables and Mutexes
//////////////////////////////////////////

var (
	// In-memory storage for DNS requests and players.
	dnsRequests    = make(map[string]*DNSRequest)
	players        = make(map[string]*Player)
	pendingActions sync.Map // Stores channels for pending DNS actions.

	// Mutexes to ensure thread-safe operations.
	dnsRequestsMu     sync.RWMutex
	playersMu         sync.RWMutex
	pendingRequestsMu sync.Mutex

	// Slice to manage pending DNS requests.
	pendingRequests []*DNSRequest
)

//////////////////////////////////////////
// Helper Functions
//////////////////////////////////////////

// generateRequestID creates a unique RequestID based on the current timestamp.
func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

// generatePlayerID creates a unique PlayerID based on the current timestamp.
func generatePlayerID() string {
	return fmt.Sprintf("player-%d", time.Now().UnixNano())
}

// getEnv retrieves an environment variable or returns a fallback default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

//////////////////////////////////////////
// HTTP Handlers
//////////////////////////////////////////

// dnsRequestHandler processes incoming DNS requests from CoreDNS.
func dnsRequestHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	dnsRequestsTotal.Inc()

	var dnsReq DNSRequest
	if err := json.NewDecoder(r.Body).Decode(&dnsReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Initialize the DNS request.
	dnsReq.RequestID = generateRequestID()
	dnsReq.Assigned = false
	dnsReq.Timestamp = time.Now()
	dnsReq.TimedOut = false // Initialize TimedOut to false

	// Create a channel to receive the player's action.
	actionChan := make(chan string, 1) // Buffered to prevent blocking.

	// Store the DNS request in the map.
	dnsRequestsMu.Lock()
	dnsRequests[dnsReq.RequestID] = &dnsReq
	dnsRequestsMu.Unlock()

	// Store the action channel for later communication.
	pendingActions.Store(dnsReq.RequestID, actionChan)

	// Add the DNS request to the pendingRequests slice.
	pendingRequestsMu.Lock()
	pendingRequests = append(pendingRequests, &dnsReq)
	pendingDNSRequests.Set(float64(len(pendingRequests)))
	pendingRequestsMu.Unlock()

	log.Printf("[RequestID: %s] Received DNS request: %v", dnsReq.RequestID, dnsReq)

	// Await the player's action or timeout after 30 seconds.
	var action string
	select {
	case action = <-actionChan:
		// Player provided an action.
	case <-time.After(30 * time.Second):
		// Timeout occurred; default to "correct" action.
		action = "correct"
		dnsReq.TimedOut = true // Mark the request as timed out
		log.Printf("[RequestID: %s] DNS request timed out after 30 seconds", dnsReq.RequestID)
	}

	// Respond to the DNS plugin with the chosen action.
	dnsResp := DNSResponse{Action: action}
	json.NewEncoder(w).Encode(dnsResp)

	// Record the request duration with the action label.
	dnsRequestLatency.With(prometheus.Labels{
		"action": action,
	}).Observe(time.Since(start).Seconds())

	// Do NOT call cleanupDNSRequest here. Allow the player additional time to submit their action.
}

// assignDNSRequestHandler assigns a pending DNS request to a player.
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

	// Check if the player already has an assigned request.
	if player.AssignedRequestID != "" {
		dnsRequestsMu.RLock()
		dnsReq, exists := dnsRequests[player.AssignedRequestID]
		dnsRequestsMu.RUnlock()
		if exists && dnsReq.Assigned && !dnsReq.TimedOut {
			// Check if the assigned request has sufficient remaining time.
			remainingTime := 30*time.Second - time.Since(dnsReq.Timestamp)
			if remainingTime > MinimumRemainingTime {
				log.Printf("[PlayerID: %s] Already assigned request %s", playerID, dnsReq.RequestID)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(dnsReq)
				playersMu.Unlock()
				return
			}
		}
		// Clear the assigned request if it's no longer valid or has timed out.
		log.Printf("[PlayerID: %s] Clearing expired or invalid assigned request %s", playerID, player.AssignedRequestID)
		player.AssignedRequestID = ""
	}
	playersMu.Unlock()

	// Assign a new DNS request from the pendingRequests slice.
	dnsReq := fetchPendingDNSRequest()
	if dnsReq == nil {
		log.Printf("[PlayerID: %s] No DNS requests available; cannot assign a DNS request", playerID)
		http.Error(w, "No DNS requests available", http.StatusNoContent)
		return
	}

	// Double-check if the DNS request is still valid and has sufficient remaining time.
	remainingTime := 30*time.Second - time.Since(dnsReq.Timestamp)
	if dnsReq.TimedOut || remainingTime <= MinimumRemainingTime {
		log.Printf("[RequestID: %s] DNS request has timed out or is too old; cannot assign to player %s", dnsReq.RequestID, playerID)
		http.Error(w, "DNS request has timed out or is too old", http.StatusGone)
		return
	}

	// Assign the DNS request to the player.
	playersMu.Lock()
	player, exists = players[playerID]
	if !exists {
		playersMu.Unlock()
		http.Error(w, "Invalid player_id", http.StatusBadRequest)
		return
	}
	dnsReq.Assigned = true
	player.AssignedRequestID = dnsReq.RequestID
	log.Printf("[PlayerID: %s] Assigned request %s", playerID, dnsReq.RequestID)
	playersMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dnsReq)
}

// submitActionHandler processes actions submitted by players.
func submitActionHandler(w http.ResponseWriter, r *http.Request) {
	var actionReq struct {
		PlayerID  string `json:"player_id"`
		RequestID string `json:"request_id"`
		Action    string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&actionReq); err != nil {
		log.Printf("Failed to decode action request: %v", err)
		http.Error(w, "Invalid request data.", http.StatusBadRequest)
		return
	}

	// Validate the player.
	playersMu.RLock()
	player, exists := players[actionReq.PlayerID]
	playersMu.RUnlock()
	if !exists {
		log.Printf("Invalid player ID: %s", actionReq.PlayerID)
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}

	// Validate the assigned request.
	playersMu.RLock()
	assignedRequestID := player.AssignedRequestID
	playersMu.RUnlock()
	if assignedRequestID != actionReq.RequestID {
		log.Printf("Player %s assigned request %s does not match submitted request %s", actionReq.PlayerID, assignedRequestID, actionReq.RequestID)
		if assignedRequestID == "" {
			http.Error(w, "The DNS request has expired or was already handled.", http.StatusBadRequest)
		} else {
			http.Error(w, "Invalid request_id for this player", http.StatusBadRequest)
		}
		return
	}

	// Validate the DNS request.
	dnsRequestsMu.RLock()
	dnsReq, exists := dnsRequests[actionReq.RequestID]
	dnsRequestsMu.RUnlock()
	if !exists || !dnsReq.Assigned {
		log.Printf("Invalid or unassigned DNS request: %s", actionReq.RequestID)
		http.Error(w, "The DNS request has expired or was already handled.", http.StatusBadRequest)
		return
	}

	// Check if the DNS request has timed out.
	if dnsReq.TimedOut {
		log.Printf("Player %s submitted action for timed-out request %s", actionReq.PlayerID, actionReq.RequestID)
		http.Error(w, "The DNS request has expired.", http.StatusBadRequest)
		return
	}

	// Update the player's score based on the submitted action.
	updatePlayerScore(actionReq.PlayerID, actionReq.Action)

	// Notify the DNS request handler of the player's action.
	notifyDNSRequestHandler(actionReq.RequestID, actionReq.Action)

	// Clear the player's assigned request.
	clearPlayerAssignment(actionReq.PlayerID)

	// Clean up the processed request.
	cleanupDNSRequest(actionReq.RequestID, actionReq.Action)

	log.Printf("Player %s submitted action '%s' for request %s", actionReq.PlayerID, actionReq.Action, actionReq.RequestID)

	w.WriteHeader(http.StatusOK)
}

// registerHandler handles player registration.
func registerHandler(w http.ResponseWriter, r *http.Request) {
	nickname := r.URL.Query().Get("nickname")
	if nickname == "" {
		http.Error(w, "Nickname is required", http.StatusBadRequest)
		return
	}

	playerID := generatePlayerID()

	// Create a new player instance.
	player := &Player{
		ID:         playerID,
		Nickname:   nickname,
		PurePoints: 0,
		EvilPoints: 0,
	}

	// Store the player in memory.
	playersMu.Lock()
	players[playerID] = player
	playerCount.Set(float64(len(players)))
	playersMu.Unlock()

	// Persist the new player to the database asynchronously.
	go func() {
		if err := db.CreatePlayer(playerID, nickname); err != nil {
			log.Printf("Warning: Failed to persist player %s to database: %v", playerID, err)
		}
	}()

	log.Printf("Registered player: %s (%s)", nickname, playerID)
	w.Write([]byte(playerID))
}

// leaderboardHandler returns the current leaderboard with pagination.
func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	type LeaderboardEntry struct {
		PlayerID     string  `json:"player_id"`
		Nickname     string  `json:"nickname"`
		PurePoints   float64 `json:"pure_points"`
		EvilPoints   float64 `json:"evil_points"`
		NetAlignment float64 `json:"net_alignment"`
	}

	// Parse pagination parameters.
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize := 50 // Fixed page size of 50 items.

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

	// Sort the leaderboard by total points in descending order.
	sort.Slice(leaderboard, func(i, j int) bool {
		totalI := leaderboard[i].PurePoints + leaderboard[i].EvilPoints
		totalJ := leaderboard[j].PurePoints + leaderboard[j].EvilPoints
		return totalI > totalJ
	})

	// Calculate pagination bounds.
	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	if startIndex >= len(leaderboard) {
		startIndex = 0
		endIndex = 0
	} else if endIndex > len(leaderboard) {
		endIndex = len(leaderboard)
	}

	// Return the paginated leaderboard slice.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard[startIndex:endIndex])
}

//////////////////////////////////////////
// Helper Functions for Handlers
//////////////////////////////////////////

// cleanupDNSRequest removes a processed DNS request from in-memory storage and updates metrics.
func cleanupDNSRequest(requestID, action string) {
	dnsRequestsMu.Lock()
	delete(dnsRequests, requestID)
	dnsRequestsMu.Unlock()

	pendingActions.Delete(requestID)
	removePendingRequest(requestID)

	// Clear the player's assigned request if it matches this requestID.
	playersMu.Lock()
	for _, player := range players {
		if player.AssignedRequestID == requestID {
			player.AssignedRequestID = ""
			log.Printf("Cleared AssignedRequestID for player %s because request %s was processed", player.ID, requestID)
			break
		}
	}
	playersMu.Unlock()
}

// removePendingRequest removes a DNS request from the pendingRequests slice by RequestID.
func removePendingRequest(requestID string) {
	pendingRequestsMu.Lock()
	defer pendingRequestsMu.Unlock()

	for i, req := range pendingRequests {
		if req.RequestID == requestID {
			pendingRequests = append(pendingRequests[:i], pendingRequests[i+1:]...)
			pendingDNSRequests.Set(float64(len(pendingRequests)))
			break
		}
	}
}

// fetchPendingDNSRequest retrieves and removes the first unassigned DNS request from the pendingRequests slice.
func fetchPendingDNSRequest() *DNSRequest {
	pendingRequestsMu.Lock()
	defer pendingRequestsMu.Unlock()

	now := time.Now()
	for i, req := range pendingRequests {
		remainingTime := 30*time.Second - now.Sub(req.Timestamp)
		if !req.Assigned && !req.TimedOut && remainingTime > MinimumRemainingTime {
			// Remove the request from the slice.
			pendingRequests = append(pendingRequests[:i], pendingRequests[i+1:]...)
			pendingDNSRequests.Set(float64(len(pendingRequests)))
			return req
		}
	}
	return nil
}

// updatePlayerScore updates the player's score based on the action taken.
func updatePlayerScore(playerID, action string) {
	playersMu.Lock()
	defer playersMu.Unlock()

	player, exists := players[playerID]
	if !exists {
		log.Printf("Player %s not found while updating score", playerID)
		return
	}

	switch action {
	case "correct":
		player.PurePoints += 1
		player.PureDelta += 1
		playerActionCounter.With(prometheus.Labels{"action": "correct"}).Inc()
	case "corrupt", "delay", "nxdomain":
		player.EvilPoints += 1
		player.EvilDelta += 1
		playerActionCounter.With(prometheus.Labels{"action": action}).Inc()
	default:
		log.Printf("Invalid action '%s' submitted by player %s", action, playerID)
	}
}

// notifyDNSRequestHandler sends the player's action back to the DNS request handler.
func notifyDNSRequestHandler(requestID, action string) {
	value, ok := pendingActions.Load(requestID)
	if ok {
		actionChan := value.(chan string)
		actionChan <- action
	} else {
		log.Printf("Action channel not found for request %s", requestID)
	}
}

// clearPlayerAssignment clears the assigned DNS request for a player.
func clearPlayerAssignment(playerID string) {
	playersMu.Lock()
	defer playersMu.Unlock()

	player, exists := players[playerID]
	if !exists {
		log.Printf("Player %s not found while clearing assignment", playerID)
		return
	}
	player.AssignedRequestID = ""
}

//////////////////////////////////////////
// Background Goroutines
//////////////////////////////////////////

// cleanupExpiredRequests periodically removes DNS requests that have expired.
func cleanupExpiredRequests() {
	for {
		time.Sleep(1 * time.Minute)
		dnsRequestsMu.Lock()
		pendingRequestsMu.Lock()
		now := time.Now()
		var expiredRequests []string

		for reqID, dnsReq := range dnsRequests {
			if now.Sub(dnsReq.Timestamp) > 5*time.Minute {
				delete(dnsRequests, reqID)
				removePendingRequest(reqID)
				expiredRequests = append(expiredRequests, reqID)
				log.Printf("[RequestID: %s] Expired DNS request cleaned up after 5 minutes", reqID)

				// Clear the player's assigned request if it matches this requestID.
				playersMu.Lock()
				for _, player := range players {
					if player.AssignedRequestID == reqID {
						player.AssignedRequestID = ""
						log.Printf("Cleared AssignedRequestID for player %s because request %s expired", player.ID, reqID)
						break
					}
				}
				playersMu.Unlock()
			}
		}

		// Update the pendingDNSRequests metric.
		pendingDNSRequests.Set(float64(len(pendingRequests)))
		pendingRequestsMu.Unlock()
		dnsRequestsMu.Unlock()

		if len(expiredRequests) > 0 {
			log.Printf("Cleaned up %d expired DNS requests", len(expiredRequests))
		}
	}
}

// syncPlayersToDatabase periodically syncs in-memory player data to SQLite.
func syncPlayersToDatabase() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		playersMu.RLock()
		for _, player := range players {
			if player.PureDelta != 0 || player.EvilDelta != 0 {
				if err := db.AddPlayerPoints(player.ID, player.PureDelta, player.EvilDelta); err != nil {
					log.Printf("Error syncing player %s to database: %v", player.ID, err)
				} else {
					// Reset deltas after successful sync.
					player.PureDelta = 0
					player.EvilDelta = 0
				}
			}
		}
		playersMu.RUnlock()
		log.Printf("Synced player deltas to database")
	}
}

//////////////////////////////////////////
// Main Function
//////////////////////////////////////////

func main() {
	// Retrieve the database path from environment variables or use the default.
	dbPath := getEnv("DB_PATH", "/litefs/gameserver.db")

	// Ensure the database directory exists.
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Printf("Warning: Failed to create database directory: %v", err)
	}

	// Initialize the database connection.
	if err := db.Initialize(dbPath); err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
	}

	// Load existing players from the database into memory.
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

	// Start the periodic database synchronization.
	go syncPlayersToDatabase()

	// Initialize the HTTP server multiplexer.
	mux := http.NewServeMux()

	// Register HTTP handlers.
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/dnsrequest", dnsRequestHandler)
	mux.HandleFunc("/submitaction", submitActionHandler)
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/assign", assignDNSRequestHandler)
	mux.HandleFunc("/leaderboard", leaderboardHandler)

	// Start the DNS request cleanup goroutine.
	go cleanupExpiredRequests()

	// Configure the HTTP server.
	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,  // Maximum duration for reading the entire request, including the body.
		WriteTimeout: 35 * time.Second, // Maximum duration before timing out writes of the response.
		IdleTimeout:  60 * time.Second, // Maximum time to wait for the next request when keep-alives are enabled.
	}

	// Channel to listen for server errors.
	serverErrors := make(chan error, 1)

	// Start the HTTP server in a separate goroutine.
	go func() {
		log.Println("Game server running on port 8080")
		serverErrors <- server.ListenAndServe()
	}()

	// Channel to listen for OS signals for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Block until a signal is received or an error occurs.
	select {
	case err := <-serverErrors:
		log.Fatalf("Could not start server: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v. Shutting down...", sig)

		// Create a context with timeout for the shutdown process.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown.
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the server: %v", err)
		}
	}
}
