package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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

	playTemplate        *template.Template
	leaderboardTemplate *template.Template
	registerTemplate    *template.Template
)

func init() {
	// Create an HTTP client with timeouts
	client = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Parse templates once at startup
	var err error
	playTemplate, err = template.ParseFiles("templates/play.html")
	if err != nil {
		log.Fatalf("Failed to parse play.html: %v", err)
	}

	leaderboardTemplate, err = template.ParseFiles("templates/leaderboard.html")
	if err != nil {
		log.Fatalf("Failed to parse leaderboard.html: %v", err)
	}

	registerTemplate, err = template.ParseFiles("templates/register.html")
	if err != nil {
		log.Fatalf("Failed to parse register.html: %v", err)
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

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get assigned DNS request: status code %d", resp.StatusCode)
		http.Error(w, "No DNS requests available. Please try again later.", http.StatusServiceUnavailable)
		return
	}

	var dnsReq DNSRequest
	err = json.NewDecoder(resp.Body).Decode(&dnsReq)
	if err != nil {
		log.Printf("Failed to decode DNS request: %v", err)
		http.Error(w, "Failed to decode DNS request.", http.StatusInternalServerError)
		return
	}

	err = playTemplate.Execute(w, dnsReq)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Failed to render template.", http.StatusInternalServerError)
		return
	}

}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	playerID := getPlayerID(w, r)

	if playerID == "" {
		// getPlayerID already handled the error response
		return
	}

	err := r.ParseForm()
	if err != nil {
		log.Printf("Failed to parse form: %v", err)
		http.Error(w, "Invalid form data.", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	requestID := r.FormValue("request_id")

	actionReq := map[string]string{
		"player_id":  playerID,
		"request_id": requestID,
		"action":     action,
	}

	data, _ := json.Marshal(actionReq)
	resp, err := client.Post("http://gameserver:8080/submitaction", "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Failed to submit action: %v", err)
		http.Error(w, "Failed to submit action.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to submit action: status code %d", resp.StatusCode)
		http.Error(w, "Failed to submit action.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/play", http.StatusSeeOther)
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := client.Get("http://gameserver:8080/leaderboard")
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

	err = leaderboardTemplate.Execute(w, leaderboard)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Failed to render template.", http.StatusInternalServerError)
		return
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Display the registration form
		err := registerTemplate.Execute(w, nil)
		if err != nil {
			log.Printf("Error executing template: %v", err)
			http.Error(w, "Failed to render template.", http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodPost {
		// Process the registration form
		err := r.ParseForm()
		if err != nil {
			log.Printf("Failed to parse form: %v", err)
			http.Error(w, "Invalid form data.", http.StatusBadRequest)
			return
		}

		nickname := r.FormValue("nickname")

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

		http.SetCookie(w, &http.Cookie{
			Name:  "player_id",
			Value: playerID,
			Path:  "/",
		})

		http.Redirect(w, r, "/play", http.StatusSeeOther)
	}
}

func getPlayerID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("player_id")
	if err == nil {
		return cookie.Value
	}

	// Redirect to registration page
	http.Redirect(w, r, "/register", http.StatusSeeOther)
	return ""
}

func main() {
	http.HandleFunc("/play", playHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/leaderboard", leaderboardHandler)
	http.HandleFunc("/register", registerHandler)
	log.Println("Web interface running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
