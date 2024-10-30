// dnsloader/main.go

package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
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

	// defaultMetricsURL is the default URL for Prometheus metrics.
	defaultMetricsURL = "http://gameserver:8080/metrics"

	// defaultTargetQueueSize is the default target size for the DNS request queue.
	defaultTargetQueueSize = 100

	// defaultAdjustInterval is the default interval for adjusting the DNS query rate.
	defaultAdjustInterval = "10s"

	// defaultCheckInterval is the default interval for checking DNS queue metrics.
	defaultCheckInterval = "5s"

	// domainsFile is the filename containing the list of domains to query.
	domainsFile = "domains.txt"
)

//////////////////////////////////////////
// Global Variables
//////////////////////////////////////////

var (
	// Configuration variables loaded from environment or defaults.
	dnsServer       string
	dnsPort         string
	metricsURL      string
	targetQueueSize int
	domains         []string
	adjustInterval  time.Duration
	checkInterval   time.Duration
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
	metricsURL = getEnv("METRICS_URL", defaultMetricsURL)

	// Parse target queue size, defaulting to 100 if parsing fails.
	if size, err := strconv.Atoi(getEnv("TARGET_QUEUE_SIZE", strconv.Itoa(defaultTargetQueueSize))); err == nil {
		targetQueueSize = size
	} else {
		targetQueueSize = defaultTargetQueueSize
	}

	// Parse adjust interval duration, defaulting to 10 seconds if parsing fails.
	if interval, err := time.ParseDuration(getEnv("ADJUST_INTERVAL", defaultAdjustInterval)); err == nil {
		adjustInterval = interval
	} else {
		adjustInterval = 10 * time.Second
	}

	// Parse check interval duration, defaulting to 5 seconds if parsing fails.
	if interval, err := time.ParseDuration(getEnv("CHECK_INTERVAL", defaultCheckInterval)); err == nil {
		checkInterval = interval
	} else {
		checkInterval = 5 * time.Second
	}
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
// DNSLoader Structure and Methods
//////////////////////////////////////////

// DNSLoader is responsible for generating and sending DNS queries to the DNS server.
// It adjusts the query rate based on the DNS server's queue metrics to maintain optimal performance.
type DNSLoader struct {
	currentRate     int           // Current DNS query rate (queries per second)
	dnsServerIP     string        // IP address of the DNS server
	dnsPort         string        // Port number of the DNS server
	metricsURL      string        // URL to fetch Prometheus metrics
	targetQueueSize int           // Desired size of the DNS request queue
	domains         []string      // List of domains to query
	stopChan        chan struct{} // Channel to signal stopping of the DNSLoader
}

// NewDNSLoader creates and initializes a new DNSLoader instance with the provided configuration.
// It sets the initial query rate and prepares the stop channel for graceful shutdown.
func NewDNSLoader(dnsServerIP, dnsPort, metricsURL string, targetQueueSize int, domains []string) *DNSLoader {
	return &DNSLoader{
		currentRate:     1, // Start with 1 query per second
		dnsServerIP:     dnsServerIP,
		dnsPort:         dnsPort,
		metricsURL:      metricsURL,
		targetQueueSize: targetQueueSize,
		domains:         domains,
		stopChan:        make(chan struct{}),
	}
}

// getQueueMetrics retrieves the current DNS queue length from the Prometheus metrics endpoint.
// It parses the Prometheus metrics to extract the 'dns_queue_length' gauge value.
func (d *DNSLoader) getQueueMetrics() (float64, error) {
	resp, err := http.Get(d.metricsURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return 0, err
	}

	if metric, ok := metrics["dns_queue_length"]; ok {
		if len(metric.Metric) > 0 && metric.Metric[0].Gauge != nil {
			return metric.Metric[0].Gauge.GetValue(), nil
		}
	}
	return 0, fmt.Errorf("queue length metric not found")
}

// adjustRate modifies the DNS query rate based on the current queue length.
// If the queue is below 80% of the target, it increases the rate by 20%.
// If the queue exceeds 120% of the target, it decreases the rate by 20%.
// This dynamic adjustment helps in maintaining optimal load on the DNS server.
func (d *DNSLoader) adjustRate() {
	queueLength, err := d.getQueueMetrics()
	if err != nil {
		log.Printf("Error getting metrics: %v", err)
		return
	}

	// Adjust rate based on queue length
	if queueLength < float64(d.targetQueueSize)*0.8 {
		d.currentRate = int(float64(d.currentRate) * 1.2)
	} else if queueLength > float64(d.targetQueueSize)*1.2 {
		d.currentRate = int(float64(d.currentRate) * 0.8)
	}

	// Ensure the rate does not fall below 1 query per second
	if d.currentRate < 1 {
		d.currentRate = 1
	}

	log.Printf("Current queue length: %.2f, Target: %d, Adjusted rate: %d/sec",
		queueLength, d.targetQueueSize, d.currentRate)
}

// sendDNSQuery constructs and sends a DNS query for the specified domain to the DNS server.
// It establishes a UDP connection, packs the DNS message, and sends it over the network.
func (d *DNSLoader) sendDNSQuery(domain string) {
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

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%s", d.dnsServerIP, d.dnsPort))
	if err != nil {
		log.Printf("Failed to connect to DNS server %s:%s - %v", d.dnsServerIP, d.dnsPort, err)
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

// Start initiates the DNSLoader's query sending process.
// It runs in a separate goroutine, continuously sending DNS queries at the current rate
// and adjusting the rate at specified intervals based on queue metrics.
func (d *DNSLoader) Start() {
	// Ticker for adjusting the query rate
	adjustTicker := time.NewTicker(adjustInterval)
	// Ticker for sending DNS queries based on the current rate
	queryTicker := time.NewTicker(time.Duration(1e9 / d.currentRate)) // Initial rate

	go func() {
		defer adjustTicker.Stop()
		defer queryTicker.Stop()

		for {
			select {
			case <-d.stopChan:
				log.Println("DNSLoader received stop signal. Exiting query loop.")
				return
			case <-adjustTicker.C:
				d.adjustRate()
				// Reset the query ticker based on the new rate
				queryTicker.Stop()
				queryTicker = time.NewTicker(time.Duration(1e9 / d.currentRate))
			case <-queryTicker.C:
				// Select a random domain to query
				domain := d.domains[rand.Intn(len(d.domains))]
				d.sendDNSQuery(domain)
			}
		}
	}()
}

// Stop signals the DNSLoader to cease sending DNS queries.
// It closes the stop channel, which gracefully terminates the query loop.
func (d *DNSLoader) Stop() {
	close(d.stopChan)
	log.Println("DNSLoader has been stopped.")
}

//////////////////////////////////////////
// Main Function
//////////////////////////////////////////

// main is the entry point of the DNSLoader application.
// It initializes configurations, loads domains, resolves DNS server IP,
// creates and starts the DNSLoader, and handles graceful shutdown on interrupt signals.
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

	// Create a new DNSLoader instance with the loaded configuration.
	loader := NewDNSLoader(
		dnsServerIP.String(),
		dnsPort,
		metricsURL,
		targetQueueSize,
		domains,
	)

	log.Printf("Starting DNSLoader with initial rate: %d queries/sec", loader.currentRate)
	loader.Start()

	// Channel to listen for interrupt signals for graceful shutdown.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Block until an interrupt signal is received.
	<-c

	log.Printf("Interrupt signal received. Shutting down DNSLoader...")
	loader.Stop()
	log.Printf("DNSLoader has been successfully shut down.")
}
