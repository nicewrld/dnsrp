// webinterface/main.go
package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DNSRequest struct {
	RequestID string `json:"request_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Class     string `json:"class"`
}

type DNSResponse struct {
	Action string `json:"action"`
}

// Global variables
var (
	client *http.Client
)

func init() {
	// Create an HTTP client with timeouts
	client = &http.Client{
		Timeout: 10 * time.Second,
	}
}

func playHandler(w http.ResponseWriter, r *http.Request) {
	// Generate or retrieve player ID (e.g., via cookie)
	playerID := getPlayerID(w, r)

	if playerID == "" {
		// getPlayerID already handled the error response
		return
	}

	// Request an assigned DNS query from the game server
	resp, err := client.Get("http://gameserver:8080/assign?player_id=" + url.QueryEscape(playerID))
	if err != nil {
		// Log the error for debugging
		log.Printf("Failed to get assigned DNS request: %v", err)
		http.Error(w, "No DNS requests available. Please try again later.", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		log.Printf("No DNS requests available for player %s", playerID)
		http.Error(w, "No DNS requests available.", http.StatusNoContent)
		return
	}

	if resp.StatusCode == http.StatusBadRequest {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		log.Printf("Failed to get DNS request: %s", bodyString)
		http.Error(w, bodyString, http.StatusBadRequest)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get assigned DNS request: status code %d", resp.StatusCode)
		http.Error(w, "Failed to get DNS request.", http.StatusInternalServerError)
		return
	}

	var dnsReq DNSRequest
	err = json.NewDecoder(resp.Body).Decode(&dnsReq)
	if err != nil {
		log.Printf("Failed to decode DNS request: %v", err)
		http.Error(w, "Failed to decode DNS request.", http.StatusInternalServerError)
		return
	}

	// Return DNS request as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dnsReq)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	playerID := getPlayerID(w, r)

	if playerID == "" {
		// getPlayerID already handled the error response
		return
	}

	var actionReq map[string]string
	err := json.NewDecoder(r.Body).Decode(&actionReq)
	if err != nil {
		log.Printf("Failed to parse request body: %v", err)
		http.Error(w, "Invalid request data.", http.StatusBadRequest)
		return
	}

	actionReq["player_id"] = playerID

	data, _ := json.Marshal(actionReq)
	resp, err := client.Post("http://gameserver:8080/submitaction", "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to submit action: %v", err)
		http.Error(w, "Failed to submit action.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		log.Printf("Failed to submit action: status code %d, response: %s", resp.StatusCode, bodyString)
		http.Error(w, bodyString, resp.StatusCode)
		return
	}

	// Return success response
	w.WriteHeader(http.StatusOK)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	// Forward the page parameter from the frontend to the gameserver
	page := r.URL.Query().Get("page")
	url := "http://gameserver:8080/leaderboard"
	if page != "" {
		url += "?page=" + page
	}
	
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Failed to get leaderboard: %v", err)
		http.Error(w, "Failed to get leaderboard.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get leaderboard: status code %d", resp.StatusCode)
		http.Error(w, "Failed to get leaderboard.", http.StatusInternalServerError)
		return
	}

	var leaderboard []struct {
		PlayerID     string  `json:"player_id"`
		Nickname     string  `json:"nickname"`
		PurePoints   float64 `json:"pure_points"`
		EvilPoints   float64 `json:"evil_points"`
		NetAlignment float64 `json:"net_alignment"`
	}

	err = json.NewDecoder(resp.Body).Decode(&leaderboard)
	if err != nil {
		log.Printf("Failed to decode leaderboard: %v", err)
		http.Error(w, "Failed to decode leaderboard.", http.StatusInternalServerError)
		return
	}

	// Return leaderboard as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Process the registration form
		var reqData map[string]string
		err := json.NewDecoder(r.Body).Decode(&reqData)
		if err != nil {
			log.Printf("Failed to parse request body: %v", err)
			http.Error(w, "Invalid request data.", http.StatusBadRequest)
			return
		}

		nickname := reqData["nickname"]
		log.Printf("Registering player with nickname: %s", nickname)

		resp, err := client.Get("http://gameserver:8080/register?nickname=" + url.QueryEscape(nickname))
		if err != nil {
			log.Printf("Failed to register player: %v", err)
			http.Error(w, "Failed to register player.", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to register player: status code %d", resp.StatusCode)
			http.Error(w, "Failed to register player.", http.StatusInternalServerError)
			return
		}

		data, _ := ioutil.ReadAll(resp.Body)
		playerID := string(data)
		log.Printf("Player registered with ID: %s", playerID)

		// Set cookie
		http.SetCookie(w, &http.Cookie{
			Name:  "player_id",
			Value: playerID,
			Path:  "/",
		})

		// Return success response
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
	}
}

func getPlayerID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("player_id")
	if err == nil {
		return cookie.Value
	}

	// Return empty string and set status code
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return ""
}

func main() {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/play", playHandler)
	mux.HandleFunc("/api/submit", submitHandler)
	mux.HandleFunc("/api/leaderboard", leaderboardHandler)
	mux.HandleFunc("/api/register", registerHandler)

	// Serve static files
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If the request is for an API endpoint, return 404
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Serve static files
		fs := http.FileServer(http.Dir("public"))
		fs.ServeHTTP(w, r)
	})

	log.Println("Web interface running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
