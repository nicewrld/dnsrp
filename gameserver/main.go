// main.go

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

type DNSRequest struct {
	RequestID string `json:"request_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Class     string `json:"class"`
	Assigned  bool   `json:"assigned"`
}

type Player struct {
	ID         string
	Nickname   string
	PurePoints float64
	EvilPoints float64
}

var (
	dnsRequests    = make(map[string]*DNSRequest) // Map of request ID to DNSRequest
	dnsQueue       = make(chan *DNSRequest, 100)
	pendingActions = sync.Map{}               // Map of request ID to action channel
	players        = make(map[string]*Player) // Map of player ID to Player
	mu             = sync.Mutex{}             // Mutex to protect dnsRequests and players
)

type DNSResponse struct {
	Action string `json:"action"`
}

func dnsRequestHandler(w http.ResponseWriter, r *http.Request) {
	var dnsReq DNSRequest
	err := json.NewDecoder(r.Body).Decode(&dnsReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dnsReq.RequestID = generateRequestID()
	dnsReq.Assigned = false

	actionChan := make(chan string)

	mu.Lock()
	dnsRequests[dnsReq.RequestID] = &dnsReq
	pendingActions.Store(dnsReq.RequestID, actionChan)
	mu.Unlock()

	dnsQueue <- &dnsReq

	log.Printf("Received DNS request: %v", dnsReq)
	log.Printf("dnsQueue length: %d", len(dnsQueue))

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

	// Clean up
	mu.Lock()
	delete(dnsRequests, dnsReq.RequestID)
	mu.Unlock()
	pendingActions.Delete(dnsReq.RequestID)
}

func assignDNSRequestHandler(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "Missing player_id", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Ensure the player exists in the players map
	player, exists := players[playerID]
	if !exists {
		players[playerID] = &Player{}
		player = players[playerID]
	}

	// Check if the player already has an assigned request
	for _, req := range dnsRequests {
		if req.Assigned && player.ID == req.RequestID {
			log.Printf("Player %s already assigned request %s", playerID, req.RequestID)
			json.NewEncoder(w).Encode(req)
			return
		}
	}

	// Assign a new request from the queue
	select {
	case dnsReq := <-dnsQueue:
		dnsReq.Assigned = true
		player.ID = dnsReq.RequestID
		log.Printf("Assigned request %s to player %s", dnsReq.RequestID, playerID)
		json.NewEncoder(w).Encode(dnsReq)
	default:
		// No DNS requests available
		log.Printf("No DNS requests available for player %s", playerID)
		http.Error(w, "No DNS requests available", http.StatusNoContent)
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Handler for players to get DNS requests
func getDNSRequestHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case dnsReq := <-dnsQueue:
		json.NewEncoder(w).Encode(dnsReq)
	default:
		// No DNS requests available
		http.Error(w, "No DNS requests available", http.StatusNoContent)
	}
}

// Handler for players to submit actions
func submitActionHandler(w http.ResponseWriter, r *http.Request) {
	var actionReq struct {
		PlayerID  string `json:"player_id"`
		RequestID string `json:"request_id"`
		Action    string `json:"action"`
	}
	err := json.NewDecoder(r.Body).Decode(&actionReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	player, exists := players[actionReq.PlayerID]
	if !exists {
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}

	dnsReq, exists := dnsRequests[actionReq.RequestID]
	if !exists || dnsReq.Assigned == false || player.ID != dnsReq.RequestID {
		http.Error(w, "Invalid request or player", http.StatusBadRequest)
		return
	}

	// Update player's score based on the action
	switch actionReq.Action {
	case "correct":
		player.PurePoints += 1
	case "corrupt", "delay", "nxdomain":
		player.EvilPoints += 1
	default:
		// Invalid action
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	value, ok := pendingActions.Load(actionReq.RequestID)
	if !ok {
		http.Error(w, "Invalid request_id", http.StatusBadRequest)
		return
	}

	actionChan := value.(chan string)
	actionChan <- actionReq.Action

	w.WriteHeader(http.StatusOK)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	type LeaderboardEntry struct {
		PlayerID     string  `json:"player_id"`
		Nickname     string  `json:"nickname"`
		PurePoints   float64 `json:"pure_points"`
		EvilPoints   float64 `json:"evil_points"`
		NetAlignment float64 `json:"net_alignment"`
	}

	mu.Lock()
	defer mu.Unlock()

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

	// Sort the leaderboard
	sort.Slice(leaderboard, func(i, j int) bool {
		return leaderboard[i].NetAlignment > leaderboard[j].NetAlignment
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}

func generatePlayerID() string {
	return fmt.Sprintf("player-%d", time.Now().UnixNano())
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	nickname := r.URL.Query().Get("nickname")
	if nickname == "" {
		http.Error(w, "Nickname is required", http.StatusBadRequest)
		return
	}

	playerID := generatePlayerID()
	mu.Lock()
	players[playerID] = &Player{
		ID:         playerID,
		Nickname:   nickname,
		PurePoints: 0,
		EvilPoints: 0,
	}
	mu.Unlock()

	log.Printf("Registered player: %s (%s)", nickname, playerID)
	w.Write([]byte(playerID))
}

func main() {
	http.HandleFunc("/dnsrequest", dnsRequestHandler)
	http.HandleFunc("/getdnsrequest", getDNSRequestHandler)
	http.HandleFunc("/submitaction", submitActionHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/assign", assignDNSRequestHandler) // Add this line
	http.HandleFunc("/leaderboard", leaderboardHandler)
	log.Println("Game server running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
