package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

var (
	dnsServer        string
	dnsPort         string
	metricsURL      string
	targetQueueSize int
	domains         []string
	adjustInterval  time.Duration
	checkInterval   time.Duration
)

func initConfig() {
	dnsServer = getEnv("DNS_SERVER", "coredns")
	dnsPort = getEnv("DNS_PORT", "5983")
	metricsURL = getEnv("METRICS_URL", "http://gameserver:8080/metrics")
	targetQueueSize, _ = strconv.Atoi(getEnv("TARGET_QUEUE_SIZE", "100"))
	adjustInterval, _ = time.ParseDuration(getEnv("ADJUST_INTERVAL", "10s"))
	checkInterval, _ = time.ParseDuration(getEnv("CHECK_INTERVAL", "5s"))
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

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

type DNSLoader struct {
	currentRate     int
	dnsServerIP     string
	dnsPort         string
	metricsURL      string
	targetQueueSize int
	domains         []string
	stopChan        chan struct{}
}

func NewDNSLoader(dnsServerIP, dnsPort, metricsURL string, targetQueueSize int, domains []string) *DNSLoader {
	return &DNSLoader{
		currentRate:     1,
		dnsServerIP:     dnsServerIP,
		dnsPort:         dnsPort,
		metricsURL:      metricsURL,
		targetQueueSize: targetQueueSize,
		domains:         domains,
		stopChan:        make(chan struct{}),
	}
}

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

	if d.currentRate < 1 {
		d.currentRate = 1
	}

	log.Printf("Current queue length: %.2f, Target: %d, Adjusted rate: %d/sec",
		queueLength, d.targetQueueSize, d.currentRate)
}

func (d *DNSLoader) sendDNSQuery(domain string) {
	msg := new(dnsMessage)
	msg.id = uint16(rand.Intn(65535))
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
		return
	}
	defer conn.Close()

	_, err = conn.Write(msg.pack())
	if err != nil {
		return
	}
}

func (d *DNSLoader) Start() {
	go func() {
		for {
			select {
			case <-d.stopChan:
				return
			default:
				// Send one query
				domain := d.domains[rand.Intn(len(d.domains))]
				d.sendDNSQuery(domain)
				
				// Wait random time between 1-60 seconds
				waitTime := time.Duration(rand.Intn(59)+1) * time.Second
				log.Printf("Sent query for %s, waiting %v before next query", domain, waitTime)
				time.Sleep(waitTime)
			}
		}
	}()
}

func (d *DNSLoader) Stop() {
	close(d.stopChan)
}

// DNS message structures (copied from stresstest)
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
	var buf []byte
	buf = append(buf, byte(msg.id>>8), byte(msg.id))
	flags := uint16(0)
	if msg.recursionDesired {
		flags |= 0x0100
	}
	buf = append(buf, byte(flags>>8), byte(flags))
	buf = append(buf, 0x00, 0x01)
	buf = append(buf, 0x00, 0x00)
	buf = append(buf, 0x00, 0x00)
	buf = append(buf, 0x00, 0x00)

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
	buf = append(buf, 0x00)
	return buf
}

func main() {
	rand.Seed(time.Now().UnixNano())
	
	initConfig()

	// Load domains
	var err error
	domains, err = loadDomains("domains.txt")
	if err != nil {
		log.Fatalf("Failed to load domains: %v", err)
	}

	// Resolve DNS server hostname
	dnsServerIP, err := net.ResolveIPAddr("ip", dnsServer)
	if err != nil {
		log.Fatalf("Failed to resolve DNS server %s: %v", dnsServer, err)
	}

	loader := NewDNSLoader(
		dnsServerIP.String(),
		dnsPort,
		metricsURL,
		targetQueueSize,
		domains,
	)

	log.Printf("Starting DNS loader with random intervals (1-60 seconds)")
	loader.Start()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	log.Printf("Shutting down...")
	loader.Stop()
}
