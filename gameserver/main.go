// gameserver/main.go
/*
 * yo this is dns server roleplay
 * 
 * what even is this?
 * ==================
 * it's like a real dns server but players can choose what happens to requests
 * you can either be:
 * - normalcore (correct responses)
 * - evilmaxxing (corrupt the data)
 * - eepypilled (add delays)
 * - gaslightpilled (pretend domains don't exist)
 *
 * how does it work?
 * ================
 * there's three main parts:
 *
 * 1. coredns plugin
 *    - yoinks real dns requests
 *    - sends them to our game
 *    - does whatever the player says
 *
 * 2. game server (you are here)
 *    - handles all the requests
 *    - keeps track of players and points
 *    - makes sure everything happens in order
 *
 * 3. players
 *    - wait for dns requests to show up
 *    - choose what to do with them
 *    - get points based on their choices
 *
 * under the hood
 * =============
 * - uses mutexes so players don't step on each other
 * - has channels to stop request flooding
 * - sync.Map for the real galaxy brain concurrent stuff
 * - graceful shutdown when things go wrong
 *
 * where's the data stored?
 * =======================
 * two places:
 * 1. ram (fast but temporary)
 *    - for active gameplay
 *    - protected by mutexes
 *    - eventually syncs to disk
 *
 * 2. sqlite (slow but forever)
 *    - saves everything important
 *    - updates every 30 seconds
 *    - uses litefs for redundancy
 *
 * metrics and stuff
 * ================
 * prometheus tracks:
 * - how many requests we're handling
 * - if the queue is getting full
 * - what players are doing
 * - if anything's broken
 *
 * made with <3 by nicewrld
 */

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
	"sync"
	"syscall" 
	"time"

	"github.com/nicewrld/gameserver/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

/*
 * PROMETHEUS METRICS
 *
 * We use several metric types to monitor game health:
 * - Counter: Monotonically increasing values (total requests)
 * - Gauge: Values that go up/down (queue size, player count)
 * - Histogram: Distribution of values (latency percentiles)
 *
 * These metrics power our Grafana dashboards and alerts.
 * Labels allow drilling down by action type.
 */
// PROMETHEUS METRICS
// =================
// These metrics provide real-time visibility into the game server's operation.
// They are exported via /metrics and can be scraped by Prometheus.

var (
	// dnsRequestsTotal tracks the absolute number of DNS requests processed
	// Used for: Capacity planning, traffic analysis, and growth tracking
	dnsRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gameserver_dns_requests_total",
		Help: "Total number of DNS requests received since server start",
	})

	// dnsRequestQueueSize monitors the current queue depth
	// Used for: Backpressure detection and auto-scaling triggers
	dnsRequestQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gameserver_dns_request_queue_size", 
		Help: "Current number of DNS requests waiting to be processed",
	})

	// dnsRequestLatency measures request processing time distributions
	// Used for: SLA monitoring and performance optimization
	// Labels: action="correct|corrupt|delay|nxdomain"
	dnsRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gameserver_dns_request_duration_seconds",
		Help:    "Time taken to process DNS requests by action type",
		Buckets: prometheus.DefBuckets, // 0.005 to 10 seconds
	}, []string{"action"})

	// playerCount tracks the number of active players
	// Used for: Capacity planning and engagement monitoring
	playerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gameserver_player_count",
		Help: "Current number of registered players in the game", 
	})

	// playerActionCounter analyzes player behavior patterns
	// Used for: Game balance analysis and cheat detection
	// Labels: action="correct|corrupt|delay|nxdomain"
	playerActionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gameserver_player_actions_total",
		Help: "Distribution of actions chosen by players",
	}, []string{"action"})
)

/*
 * CORE DATA STRUCTURES
 *
 * The game revolves around three key types:
 *
 * 1. DNSRequest - Represents an incoming query from CoreDNS
 *    The RequestID field enables request deduplication and tracking
 *    Timestamp helps with request expiration/cleanup
 *
 * 2. DNSResponse - The action to take on a DNS request
 *    Currently just an action string, but extensible for future features
 *
 * 3. Player - Tracks a player's state and score
 *    Uses separate point types to enable different gameplay strategies
 *    Deltas track changes between DB syncs for efficient updates
 */

// DNSRequest captures all relevant fields from CoreDNS queries
type DNSRequest struct {
	RequestID string    `json:"request_id"` // Unique ID for tracking
	Name      string    `json:"name"`       // Query domain name
	Type      string    `json:"type"`       // Query type (A, AAAA, etc)
	Class     string    `json:"class"`      // Query class (usually IN)
	Assigned  bool      `json:"assigned"`   // Whether a player owns this
	Timestamp time.Time `json:"timestamp"`  // When request was received
}

// DNSResponse tells CoreDNS how to handle a query
type DNSResponse struct {
	Action string `json:"action"` // correct/corrupt/delay/nxdomain
}

// Player tracks both game state and scoring
type Player struct {
	ID                string   // Unique player identifier
	Nickname          string   // Display name
	PurePoints        float64  // Points from correct responses
	EvilPoints        float64  // Points from manipulated responses
	PureDelta         float64  // Pure point changes pending DB sync
	EvilDelta         float64  // Evil point changes pending DB sync
	AssignedRequestID string   // Current request being handled
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
		player.PureDelta += 1
		playerActionCounter.With(prometheus.Labels{"action": "correct"}).Inc()
	case "corrupt", "delay", "nxdomain":
		player.EvilPoints += 1
		player.EvilDelta += 1
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
			if player.PureDelta != 0 || player.EvilDelta != 0 {
				if err := db.AddPlayerPoints(player.ID, player.PureDelta, player.EvilDelta); err != nil {
					log.Printf("Error syncing player %s to database: %v", player.ID, err)
				} else {
					// Reset deltas after successful sync
					player.PureDelta = 0
					player.EvilDelta = 0
				}
			}
		}
		playersMu.RUnlock()
		log.Printf("Synced player deltas to database")
	}
}

// getEnv retrieves an environment variable with a fallback default value
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	// Get database path from environment variable
	dbPath := getEnv("DB_PATH", "/litefs/gameserver.db")
	
	// Ensure database directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Printf("Warning: Failed to create database directory: %v", err)
	}

	// Initialize database
	if err := db.Initialize(dbPath); err != nil {
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
