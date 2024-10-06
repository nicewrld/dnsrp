package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
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

func playHandler(w http.ResponseWriter, r *http.Request) {
	// Generate or retrieve player ID (e.g., via cookie)
	playerID := getPlayerID(w, r)

	if playerID == "" {
		// getPlayerID already handled the error response
		return
	}

	// Request an assigned DNS query from the game server
	resp, err := http.Get("http://gameserver:8080/assign?player_id=" + playerID)
	if err != nil || resp.StatusCode != http.StatusOK {
		// Log the error for debugging
		log.Printf("Failed to get assigned DNS request: %v", err)
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

	tmpl := template.Must(template.ParseFiles("templates/play.html"))
	err = tmpl.Execute(w, dnsReq)
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

	r.ParseForm()
	action := r.FormValue("action")
	requestID := r.FormValue("request_id")

	actionReq := map[string]string{
		"player_id":  playerID,
		"request_id": requestID,
		"action":     action,
	}

	data, _ := json.Marshal(actionReq)
	resp, err := http.Post("http://gameserver:8080/submitaction", "application/json", bytes.NewBuffer(data))
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Failed to submit action: %v", err)
		http.Error(w, "Failed to submit action.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/play", http.StatusSeeOther)
}

func main() {

	http.HandleFunc("/play", playHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/leaderboard", leaderboardHandler)
	http.HandleFunc("/register", registerHandler)
	log.Println("Web interface running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func leaderboardHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := http.Get("http://gameserver:8080/leaderboard")
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Failed to get leaderboard: %v", err)
		http.Error(w, "Failed to get leaderboard.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

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

	tmpl := template.Must(template.ParseFiles("templates/leaderboard.html"))
	err = tmpl.Execute(w, leaderboard)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Failed to render template.", http.StatusInternalServerError)
		return
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Display the registration form
		tmpl := template.Must(template.ParseFiles("templates/register.html"))
		tmpl.Execute(w, nil)
	} else if r.Method == http.MethodPost {
		// Process the registration form
		r.ParseForm()
		nickname := r.FormValue("nickname")

		resp, err := http.Get("http://gameserver:8080/register?nickname=" + url.QueryEscape(nickname))
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("Failed to register player: %v", err)
			http.Error(w, "Failed to register player.", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
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
