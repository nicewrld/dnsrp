// dns-server-roleplay/webapp/main.go
package main

import (
	"log"
	"net/http"
)

func main() {
	// Initialize Redis client
	initRedis()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/action", actionHandler)
	http.HandleFunc("/leaderboard", leaderboardHandler)

	log.Println("Web server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
