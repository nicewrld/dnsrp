// dnsloader/main.go

package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"
)

//////////////////////////////////////////
// Constants
//////////////////////////////////////////

const (
	// dnsTypeA represents the DNS query type A (IPv4 address).
	dnsTypeA = 1

	// dnsClassIN represents the DNS class IN (Internet).
	dnsClassIN = 1

	// defaultDNSServer is the default DNS server hostname.
	defaultDNSServer = "coredns"

	// defaultDNSPort is the default DNS server port.
	defaultDNSPort = "5983"

	// domainsFile is the filename containing the list of domains to query.
	domainsFile = "domains.txt"
)

//////////////////////////////////////////
// Global Variables
//////////////////////////////////////////

var (
	// Configuration variables loaded from environment or defaults.
	dnsServer string
	dnsPort   string
	domains   []string
)

//////////////////////////////////////////
// Helper Functions
//////////////////////////////////////////

// getEnv retrieves an environment variable or returns a fallback default value.
// It ensures that configuration can be overridden via environment variables.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// initConfig initializes configuration variables by loading them from environment variables
// or falling back to predefined default values. It ensures flexibility and ease of configuration.
func initConfig() {
	dnsServer = getEnv("DNS_SERVER", defaultDNSServer)
	dnsPort = getEnv("DNS_PORT", defaultDNSPort)
}

// loadDomains reads a list of domains from a specified file.
// It returns a slice of domain strings or an error if the file cannot be read.
// This allows the DNS loader to query a predefined set of domains.
func loadDomains(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []string
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

//////////////////////////////////////////
// DNS Message Structures
//////////////////////////////////////////

// dnsMessage represents a simplified DNS query message.
type dnsMessage struct {
	id               uint16
	recursionDesired bool
	question         []dnsQuestion
}

// dnsQuestion represents a DNS question section.
type dnsQuestion struct {
	name   string
	qtype  uint16
	qclass uint16
}

// pack serializes the dnsMessage into a byte slice suitable for sending over the network.
// It follows the DNS protocol format for message packing.
func (msg *dnsMessage) pack() []byte {
	var buf []byte
	// Transaction ID
	buf = append(buf, byte(msg.id>>8), byte(msg.id))
	// Flags
	flags := uint16(0)
	if msg.recursionDesired {
		flags |= 0x0100
	}
	buf = append(buf, byte(flags>>8), byte(flags))
	// Questions, Answer RRs, Authority RRs, Additional RRs
	buf = append(buf, 0x00, 0x01) // QDCOUNT: 1 question
	buf = append(buf, 0x00, 0x00) // ANCOUNT: 0
	buf = append(buf, 0x00, 0x00) // NSCOUNT: 0
	buf = append(buf, 0x00, 0x00) // ARCOUNT: 0

	// Question Section
	for _, q := range msg.question {
		buf = append(buf, packDomainName(q.name)...)
		buf = append(buf, byte(q.qtype>>8), byte(q.qtype))
		buf = append(buf, byte(q.qclass>>8), byte(q.qclass))
	}

	return buf
}

// packDomainName converts a domain name into DNS message format.
// It splits the domain by dots and prefixes each label with its length.
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

//////////////////////////////////////////
// DNS Query Function
//////////////////////////////////////////

// sendDNSQuery constructs and sends a DNS query for the specified domain to the DNS server.
// It establishes a UDP connection, packs the DNS message, and sends it over the network.
func sendDNSQuery(dnsServerIP string, dnsPort string, domain string) {
	msg := new(dnsMessage)
	msg.id = uint16(rand.Intn(65535)) // Random transaction ID
	msg.recursionDesired = true
	msg.question = []dnsQuestion{
		{
			name:   domain,
			qtype:  dnsTypeA,
			qclass: dnsClassIN,
		},
	}

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%s", dnsServerIP, dnsPort))
	if err != nil {
		log.Printf("Failed to connect to DNS server %s:%s - %v", dnsServerIP, dnsPort, err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(msg.pack())
	if err != nil {
		log.Printf("Failed to send DNS query for domain %s - %v", domain, err)
		return
	}

	log.Printf("Sent DNS query for domain: %s", domain)
}

//////////////////////////////////////////
// Main Function
//////////////////////////////////////////

// main is the entry point of the DNSLoader application.
// It initializes configurations, loads domains, resolves DNS server IP,
// and sends DNS queries at random intervals between 1 and 10 seconds.
func main() {
	// Seed the random number generator for DNS query randomness.
	rand.Seed(time.Now().UnixNano())

	// Initialize configuration from environment variables or defaults.
	initConfig()

	// Load domains from the specified file.
	var err error
	domains, err = loadDomains(domainsFile)
	if err != nil {
		log.Fatalf("Failed to load domains from %s: %v", domainsFile, err)
	}
	log.Printf("Loaded %d domains from %s", len(domains), domainsFile)

	// Resolve the DNS server hostname to an IP address.
	dnsServerIP, err := net.ResolveIPAddr("ip", dnsServer)
	if err != nil {
		log.Fatalf("Failed to resolve DNS server %s: %v", dnsServer, err)
	}
	log.Printf("Resolved DNS server %s to IP %s", dnsServer, dnsServerIP.String())

	// Channel to listen for interrupt signals for graceful shutdown.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Run the loop in a separate goroutine
	go func() {
		for {
			// Wait for a random duration between 1 and 10 seconds
			sleepDuration := time.Duration(rand.Intn(2)+1) * time.Second
			time.Sleep(sleepDuration)

			// Select a random domain to query
			domain := domains[rand.Intn(len(domains))]
			sendDNSQuery(dnsServerIP.String(), dnsPort, domain)
		}
	}()

	// Block until an interrupt signal is received.
	<-c

	log.Printf("Interrupt signal received. Shutting down...")
}
