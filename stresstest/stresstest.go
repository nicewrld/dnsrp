package main

import (
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Configuration variables
var (
	numPlayers       int
	maxWorkers       int
	numThreads       int
	startupDelay     int
	dnsServer        string
	dnsPort          string
	webInterfaceHost string
	domains          []string
)

// Initialize configuration from environment variables
func initConfig() {
	numPlayers, _ = strconv.Atoi(getEnv("NUM_PLAYERS", "500"))
	maxWorkers, _ = strconv.Atoi(getEnv("MAX_WORKERS", "100"))
	numThreads, _ = strconv.Atoi(getEnv("NUM_THREADS", "100"))
	startupDelay, _ = strconv.Atoi(getEnv("STARTUP_DELAY", "30"))
	dnsServer = getEnv("DNS_SERVER", "coredns")
	dnsPort = getEnv("DNS_PORT", "5983")
	webInterfaceHost = getEnv("WEB_INTERFACE_HOST", "webinterface:8081")
}

// Utility function to get environment variables with defaults
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Load domains from file
func loadDomains(filename string) ([]string, error) {
	var domains []string
	file, err := os.Open(filename)
	if err != nil {
		return domains, err
	}
	defer file.Close()

	var domain string
	for {
		_, err := fmt.Fscanln(file, &domain)
		if err != nil {
			break
		}
		domains = append(domains, domain)
	}
	return domains, nil
}

// DNS Stress Test Functions

func queryDomain(domain string, dnsServerIP string, dnsPort string) {
	dialer := &net.Dialer{
		Timeout: 2 * time.Second,
	}
	conn, err := dialer.Dial("udp", dnsServerIP+":"+dnsPort)
	if err != nil {
		return
	}
	defer conn.Close()

	// Create a random DNS query ID
	id := uint16(rand.Intn(65535))

	// Build the DNS request message
	msg := new(dnsMessage)
	msg.id = id
	msg.recursionDesired = true
	msg.question = []dnsQuestion{
		{
			name:   domain,
			qtype:  dnsTypeA,
			qclass: dnsClassIN,
		},
	}
	data := msg.pack()

	_, err = conn.Write(data)
	if err != nil {
		return
	}

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read the response
	buf := make([]byte, 512)
	_, err = conn.Read(buf)
	if err != nil {
		return
	}

	// Ignore the response for stress testing
}

func dnsWorker(dnsServerIP string, dnsPort string, wg *sync.WaitGroup) {
	defer wg.Done()

	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(1000)))

	for {
		domain := domains[rand.Intn(len(domains))]
		queryDomain(domain, dnsServerIP, dnsPort)
		// Sleep for a random duration to add randomness
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	}
}

// Player Simulation Functions

func simulatePlayer(playerNumber int, sem chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() { <-sem }() // Release the semaphore

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Register the player once
	playerID, err := registerPlayer(client, playerNumber)
	if err != nil {
		log.Printf("Player %d: Failed to register - %v", playerNumber, err)
		return // Exit the goroutine if registration fails
	}
	log.Printf("Player %d: Registered successfully with PlayerID %s", playerNumber, playerID)

	// Proceed to play the game continuously
	playGame(client, playerID, playerNumber)
}

func registerPlayer(client *http.Client, playerNumber int) (string, error) {
	// Generate a random nickname
	nickname := fmt.Sprintf("Player%d_%s", playerNumber, randomString(5))

	// Prepare form data
	data := fmt.Sprintf("nickname=%s", url.QueryEscape(nickname))
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/register", webInterfaceHost), strings.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registration failed with status code %d", resp.StatusCode)
	}

	// Extract player_id cookie
	var playerID string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "player_id" {
			playerID = cookie.Value
			break
		}
	}
	if playerID == "" {
		return "", fmt.Errorf("failed to get player_id cookie")
	}

	// Initialize cookie jar and set the cookie
	jar := newSimpleJar()
	u, _ := url.Parse(fmt.Sprintf("http://%s", webInterfaceHost))
	jar.SetCookies(u, []*http.Cookie{
		{
			Name:  "player_id",
			Value: playerID,
			Path:  "/",
		},
	})
	client.Jar = jar

	return playerID, nil
}

func playGame(client *http.Client, playerID string, playerNumber int) {
	for {
		err := func() error {
			// Get assigned DNS request
			req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/play", webInterfaceHost), nil)
			if err != nil {
				return err
			}
			// The cookie is already set in client.Jar
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("Player %d: No DNS requests available. Waiting...", playerNumber)
				time.Sleep(5 * time.Second)
				return nil
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			// Parse HTML to extract request_id
			requestID, err := extractRequestID(string(body))
			if err != nil {
				log.Printf("Player %d: Failed to find request_id.", playerNumber)
				time.Sleep(5 * time.Second)
				return nil
			}

			// Randomly select an action
			action := randomAction()

			// Submit action
			data := fmt.Sprintf("action=%s&request_id=%s", url.QueryEscape(action), url.QueryEscape(requestID))
			submitReq, err := http.NewRequest("POST", fmt.Sprintf("http://%s/submit", webInterfaceHost), strings.NewReader(data))
			if err != nil {
				return err
			}
			submitReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			// Cookie is managed by client.Jar
			submitResp, err := client.Do(submitReq)
			if err != nil {
				return err
			}
			defer submitResp.Body.Close()

			if submitResp.StatusCode != http.StatusOK && submitResp.StatusCode != http.StatusSeeOther {
				log.Printf("Player %d: Failed to submit action.", playerNumber)
				time.Sleep(5 * time.Second)
				return nil
			}

			// Sleep to simulate think time
			time.Sleep(time.Duration(rand.Intn(1500)+500) * time.Millisecond)
			return nil
		}()

		if err != nil {
			log.Printf("Player %d: Error during gameplay - %v", playerNumber, err)
			time.Sleep(5 * time.Second)
			// Optionally, you can choose to break the loop or continue
			continue
		}
	}
}

// SimpleJar implements the http.CookieJar interface
type SimpleJar struct {
	mu      sync.Mutex
	cookies map[string][]*http.Cookie
}

func newSimpleJar() *SimpleJar {
	return &SimpleJar{
		cookies: make(map[string][]*http.Cookie),
	}
}

func (jar *SimpleJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.mu.Lock()
	defer jar.mu.Unlock()
	jar.cookies[u.Host] = cookies
}

func (jar *SimpleJar) Cookies(u *url.URL) []*http.Cookie {
	jar.mu.Lock()
	defer jar.mu.Unlock()
	return jar.cookies[u.Host]
}

// Utility Functions

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func randomAction() string {
	actions := []string{
		"correct", "correct", "correct", "correct", "correct",
		"correct", "correct", "correct", "correct", "correct",
		"correct", "correct", "correct", "correct", "correct",
		"correct", "correct", "correct", "correct", "correct",
		"corrupt", "corrupt", "corrupt", "corrupt",
		"delay",
		"nxdomain",
	}
	return actions[rand.Intn(len(actions))]
}

func extractRequestID(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	requestID, exists := doc.Find("input[name='request_id']").Attr("value")
	if !exists {
		return "", fmt.Errorf("request_id not found")
	}
	return html.UnescapeString(requestID), nil
}

// DNS message structures

const (
	dnsTypeA   = 1
	dnsClassIN = 1
)

type dnsMessage struct {
	id               uint16
	recursionDesired bool
	question         []dnsQuestion
}

type dnsQuestion struct {
	name   string
	qtype  uint16
	qclass uint16
}

func (msg *dnsMessage) pack() []byte {
	// Simplified DNS message packing for query
	var buf []byte

	// Header
	buf = append(buf, byte(msg.id>>8), byte(msg.id))
	flags := uint16(0)
	if msg.recursionDesired {
		flags |= 0x0100
	}
	buf = append(buf, byte(flags>>8), byte(flags))
	buf = append(buf, 0x00, 0x01) // QDCOUNT
	buf = append(buf, 0x00, 0x00) // ANCOUNT
	buf = append(buf, 0x00, 0x00) // NSCOUNT
	buf = append(buf, 0x00, 0x00) // ARCOUNT

	// Question
	for _, q := range msg.question {
		buf = append(buf, packDomainName(q.name)...)
		buf = append(buf, byte(q.qtype>>8), byte(q.qtype))
		buf = append(buf, byte(q.qclass>>8), byte(q.qclass))
	}

	return buf
}

func packDomainName(name string) []byte {
	var buf []byte
	parts := strings.Split(name, ".")
	for _, part := range parts {
		buf = append(buf, byte(len(part)))
		buf = append(buf, []byte(part)...)
	}
	buf = append(buf, 0x00) // End of domain name
	return buf
}

// Main Function

func main() {
	// Initialize configuration
	initConfig()

	// Wait for services to be ready
	log.Printf("Stress Test: Waiting %d seconds for services to be ready...", startupDelay)
	time.Sleep(time.Duration(startupDelay) * time.Second)

	// Load domains
	var err error
	domains, err = loadDomains("domains.txt")
	if err != nil {
		log.Fatalf("Failed to load domains: %v", err)
	}

	// Resolve DNS server hostname to IP address
	dnsServerIP, err := net.ResolveIPAddr("ip", dnsServer)
	if err != nil {
		log.Fatalf("Failed to resolve DNS server hostname %s: %v", dnsServer, err)
	}
	log.Printf("Resolved DNS server %s to %s", dnsServer, dnsServerIP.String())

	// Start DNS Stress Test Workers
	var dnsWg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		dnsWg.Add(1)
		go dnsWorker(dnsServerIP.String(), dnsPort, &dnsWg)
	}

	// Start Player Simulation Workers
	var playerWg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	for playerNumber := 0; playerNumber < numPlayers; playerNumber++ {
		sem <- struct{}{}
		playerWg.Add(1)
		go simulatePlayer(playerNumber, sem, &playerWg)
	}

	// Wait indefinitely
	playerWg.Wait()
	dnsWg.Wait()
}
