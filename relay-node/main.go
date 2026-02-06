package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// RelayConfig configuration for relay node
type RelayConfig struct {
	ListenPort    int      `yaml:"listen_port"`
	NodeID        string   `yaml:"node_id"`
	NextHops      []string `yaml:"next_hops"`      // Next relay nodes or gateway
	PrevHops      []string `yaml:"prev_hops"`      // Previous relay nodes or operational nodes
	GatewayURL    string   `yaml:"gateway_url"`    // If this is the final relay before gateway
	AuthToken     string   `yaml:"auth_token"`     // Token for gateway authentication
	Secret        string   `yaml:"secret"`         // Secret for node authentication
	TrafficMixing bool     `yaml:"traffic_mixing"`
	RotationTime  int      `yaml:"rotation_time"`  // seconds between route rotations
}

// RelayNode provides isolation between gateway and operational nodes
type RelayNode struct {
	config        RelayConfig
	client        *http.Client
	mu            sync.RWMutex
	currentHopIdx int
	trafficBuffer []RelayTraffic
}

// RelayTraffic represents traffic passing through relay
type RelayTraffic struct {
	RequestID string
	Data      []byte
	Timestamp time.Time
	FromNode  string
}

// NewRelayNode creates a new relay node instance
func NewRelayNode(configPath string) (*RelayNode, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config RelayConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	relay := &RelayNode{
		config: config,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		trafficBuffer: make([]RelayTraffic, 0),
	}

	// Start route rotation if configured
	if config.RotationTime > 0 {
		go relay.rotateRoutes()
	}

	// Register with gateway if this is the final relay
	if config.GatewayURL != "" && config.AuthToken == "" {
		go relay.registerWithGateway()
	}

	return relay, nil
}

// handleRelay receives and forwards traffic
func (r *RelayNode) handleRelay(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the relay data
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	fromNode := req.Header.Get("X-From-Node")
	requestID := req.Header.Get("X-Request-ID")

	log.Printf("Relay received traffic from %s (request: %s)", fromNode, requestID)

	// Add to traffic buffer if mixing enabled
	if r.config.TrafficMixing {
		r.mu.Lock()
		r.trafficBuffer = append(r.trafficBuffer, RelayTraffic{
			RequestID: requestID,
			Data:      body,
			Timestamp: time.Now(),
			FromNode:  fromNode,
		})
		r.mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Traffic queued"))
		return
	}

	// Forward immediately
	if err := r.forwardTraffic(body, requestID, fromNode); err != nil {
		http.Error(w, "Forward failed", http.StatusInternalServerError)
		log.Printf("Forward error: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Traffic relayed"))
}

// forwardTraffic sends traffic to next hop
func (r *RelayNode) forwardTraffic(data []byte, requestID, fromNode string) error {
	// Determine next hop
	var targetURL string
	
	if r.config.GatewayURL != "" {
		// This is the final relay before gateway
		targetURL = r.config.GatewayURL
	} else {
		// Select next relay node
		r.mu.Lock()
		nextHop := r.config.NextHops[r.currentHopIdx]
		r.mu.Unlock()
		targetURL = fmt.Sprintf("http://%s/relay", nextHop)
	}

	// Create request
	httpReq, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", requestID)
	httpReq.Header.Set("X-From-Node", r.config.NodeID)
	
	// Add authentication if forwarding to gateway
	if r.config.GatewayURL != "" && r.config.AuthToken != "" {
		httpReq.Header.Set("X-Node-ID", r.config.NodeID)
		httpReq.Header.Set("X-Auth-Token", r.config.AuthToken)
	}

	// Send request
	resp, err := r.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("next hop returned status %d", resp.StatusCode)
	}

	log.Printf("Forwarded request %s to %s", requestID, targetURL)
	return nil
}

// processBufferedTraffic handles batched traffic
func (r *RelayNode) processBufferedTraffic() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		if len(r.trafficBuffer) == 0 {
			r.mu.Unlock()
			continue
		}

		buffer := make([]RelayTraffic, len(r.trafficBuffer))
		copy(buffer, r.trafficBuffer)
		r.trafficBuffer = r.trafficBuffer[:0]
		r.mu.Unlock()

		log.Printf("Processing buffered traffic: %d items", len(buffer))

		for _, traffic := range buffer {
			go func(t RelayTraffic) {
				if err := r.forwardTraffic(t.Data, t.RequestID, t.FromNode); err != nil {
					log.Printf("Buffered forward error for %s: %v", t.RequestID, err)
				}
			}(traffic)
		}
	}
}

// rotateRoutes periodically changes routing paths
func (r *RelayNode) rotateRoutes() {
	ticker := time.NewTicker(time.Duration(r.config.RotationTime) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if len(r.config.NextHops) <= 1 {
			continue
		}

		r.mu.Lock()
		r.currentHopIdx = (r.currentHopIdx + 1) % len(r.config.NextHops)
		r.mu.Unlock()

		log.Printf("Rotated to next hop index %d", r.currentHopIdx)
	}
}

// registerWithGateway obtains authentication token from gateway
func (r *RelayNode) registerWithGateway() {
	// Wait a bit before registering
	time.Sleep(2 * time.Second)

	regURL := r.config.GatewayURL + "/register"
	
	regData := map[string]string{
		"node_id": r.config.NodeID,
		"secret":  r.config.Secret,
	}

	body, err := json.Marshal(regData)
	if err != nil {
		log.Printf("Registration marshal error: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, regURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("Registration request error: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		log.Printf("Registration failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Registration returned status %d", resp.StatusCode)
		return
	}

	var regResp struct {
		NodeID string `json:"node_id"`
		Token  string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		log.Printf("Registration response error: %v", err)
		return
	}

	r.mu.Lock()
	r.config.AuthToken = regResp.Token
	r.mu.Unlock()

	log.Printf("Successfully registered with gateway, token received")
}

// healthCheck endpoint
func (r *RelayNode) healthCheck(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	bufferSize := len(r.trafficBuffer)
	hasToken := r.config.AuthToken != ""
	r.mu.RUnlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "healthy",
		"role":             "relay-node",
		"node_id":          r.config.NodeID,
		"buffered_traffic": bufferSize,
		"registered":       hasToken,
		"next_hops":        len(r.config.NextHops),
		"time":             time.Now().Format(time.RFC3339),
	})
}

// Start begins the relay node server
func (r *RelayNode) Start() error {
	http.HandleFunc("/relay", r.handleRelay)
	http.HandleFunc("/health", r.healthCheck)

	// Start traffic buffer processor if mixing enabled
	if r.config.TrafficMixing {
		go r.processBufferedTraffic()
	}

	addr := fmt.Sprintf(":%d", r.config.ListenPort)
	log.Printf("Relay node %s starting on %s", r.config.NodeID, addr)
	log.Printf("Next hops: %v", r.config.NextHops)
	
	return http.ListenAndServe(addr, nil)
}

func main() {
	configPath := "config/relay.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	relay, err := NewRelayNode(configPath)
	if err != nil {
		log.Fatalf("Failed to create relay: %v", err)
	}

	if err := relay.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
