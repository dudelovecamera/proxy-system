package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// GatewayConfig configuration for Starlink gateway
type GatewayConfig struct {
	ListenPort         int      `yaml:"listen_port"`
	AuthenticatedNodes []string `yaml:"authenticated_nodes"`
	Anonymization      struct {
		TrafficMixing      bool `yaml:"traffic_mixing"`
		SourceRotation     bool `yaml:"source_rotation"`
		MACRandomization   bool `yaml:"mac_randomization"`
		TimingJitter       int  `yaml:"timing_jitter"` // milliseconds
	} `yaml:"anonymization"`
	Isolation struct {
		HideGatewayIP  bool `yaml:"hide_gateway_ip"`
		UseRelayNodes  bool `yaml:"use_relay_nodes"`
	} `yaml:"isolation"`
	NodeTokens map[string]string `yaml:"-"` // Node authentication tokens
}

// TrafficBatch aggregates traffic from multiple nodes
type TrafficBatch struct {
	Requests  []TrafficRequest
	Timestamp time.Time
	BatchID   string
}

// TrafficRequest represents a proxied request
type TrafficRequest struct {
	RequestID   string
	NodeID      string
	TargetURL   string
	Method      string
	Body        []byte
	Headers     map[string]string
	ReceivedAt  time.Time
}

// StarlinkGateway provides internet access with anonymization
type StarlinkGateway struct {
	config        GatewayConfig
	trafficBatch  []TrafficRequest
	mu            sync.RWMutex
	batchTicker   *time.Ticker
	client        *http.Client
}

// NewStarlinkGateway creates a new gateway instance
func NewStarlinkGateway(configPath string) (*StarlinkGateway, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config GatewayConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Generate authentication tokens for nodes
	config.NodeTokens = make(map[string]string)
	for _, nodeID := range config.AuthenticatedNodes {
		token := generateToken()
		config.NodeTokens[nodeID] = token
		log.Printf("Generated token for node %s: %s", nodeID, token)
	}

	gateway := &StarlinkGateway{
		config:       config,
		trafficBatch: make([]TrafficRequest, 0),
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				// Configure to rotate source IPs if multiple interfaces available
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
	}

	// Start traffic batching if mixing is enabled
	if config.Anonymization.TrafficMixing {
		gateway.batchTicker = time.NewTicker(5 * time.Second)
		go gateway.processBatches()
	}

	return gateway, nil
}

// handleProxyRequest receives requests from relay nodes
func (g *StarlinkGateway) handleProxyRequest(w http.ResponseWriter, r *http.Request) {
	// Authenticate node
	nodeID := r.Header.Get("X-Node-ID")
	token := r.Header.Get("X-Auth-Token")
	
	if !g.authenticateNode(nodeID, token) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Printf("Authentication failed for node %s", nodeID)
		return
	}

	// Parse request
	var proxyReq struct {
		RequestID string            `json:"request_id"`
		TargetURL string            `json:"target_url"`
		Method    string            `json:"method"`
		Body      []byte            `json:"body"`
		Headers   map[string]string `json:"headers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&proxyReq); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	trafficReq := TrafficRequest{
		RequestID:  proxyReq.RequestID,
		NodeID:     nodeID,
		TargetURL:  proxyReq.TargetURL,
		Method:     proxyReq.Method,
		Body:       proxyReq.Body,
		Headers:    proxyReq.Headers,
		ReceivedAt: time.Now(),
	}

	// Add timing jitter
	if g.config.Anonymization.TimingJitter > 0 {
		jitter := time.Duration(g.config.Anonymization.TimingJitter) * time.Millisecond
		time.Sleep(jitter)
	}

	if g.config.Anonymization.TrafficMixing {
		// Add to batch for later processing
		g.mu.Lock()
		g.trafficBatch = append(g.trafficBatch, trafficReq)
		g.mu.Unlock()
		
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "queued",
			"request_id": proxyReq.RequestID,
		})
	} else {
		// Process immediately
		response, err := g.performProxyRequest(trafficReq)
		if err != nil {
			http.Error(w, "Proxy error", http.StatusInternalServerError)
			log.Printf("Proxy error: %v", err)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write(response)
	}
}

// authenticateNode verifies node credentials
func (g *StarlinkGateway) authenticateNode(nodeID, token string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	expectedToken, exists := g.config.NodeTokens[nodeID]
	return exists && expectedToken == token
}

// processBatches handles batched traffic mixing
func (g *StarlinkGateway) processBatches() {
	for range g.batchTicker.C {
		g.mu.Lock()
		if len(g.trafficBatch) == 0 {
			g.mu.Unlock()
			continue
		}

		batch := make([]TrafficRequest, len(g.trafficBatch))
		copy(batch, g.trafficBatch)
		g.trafficBatch = g.trafficBatch[:0] // Clear batch
		g.mu.Unlock()

		log.Printf("Processing batch of %d requests", len(batch))

		// Process each request in the batch
		for _, req := range batch {
			go func(r TrafficRequest) {
				_, err := g.performProxyRequest(r)
				if err != nil {
					log.Printf("Batch request error for %s: %v", r.RequestID, err)
				}
			}(req)
		}
	}
}

// performProxyRequest makes the actual HTTP request to the internet
func (g *StarlinkGateway) performProxyRequest(trafficReq TrafficRequest) ([]byte, error) {
	// Create HTTP request
	req, err := http.NewRequest(
		trafficReq.Method,
		trafficReq.TargetURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}

	// Set headers (remove internal headers)
	for k, v := range trafficReq.Headers {
		if k != "X-Node-ID" && k != "X-Auth-Token" {
			req.Header.Set(k, v)
		}
	}

	// Perform request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body := make([]byte, 0)
	// In production, read the actual response body
	
	log.Printf("Proxied request %s to %s", trafficReq.RequestID, trafficReq.TargetURL)
	return body, nil
}

// handleNodeRegistration allows new nodes to register
func (g *StarlinkGateway) handleNodeRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var regReq struct {
		NodeID string `json:"node_id"`
		Secret string `json:"secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&regReq); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify secret (in production, use proper authentication)
	// For now, check if node is in authenticated list
	authorized := false
	for _, nodeID := range g.config.AuthenticatedNodes {
		if nodeID == regReq.NodeID {
			authorized = true
			break
		}
	}

	if !authorized {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate token
	token := generateToken()
	
	g.mu.Lock()
	g.config.NodeTokens[regReq.NodeID] = token
	g.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"node_id": regReq.NodeID,
		"token":   token,
	})

	log.Printf("Registered node: %s", regReq.NodeID)
}

// healthCheck endpoint
func (g *StarlinkGateway) healthCheck(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()
	batchSize := len(g.trafficBatch)
	nodeCount := len(g.config.NodeTokens)
	g.mu.RUnlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "healthy",
		"role":             "starlink-gateway",
		"queued_requests":  batchSize,
		"registered_nodes": nodeCount,
		"traffic_mixing":   g.config.Anonymization.TrafficMixing,
		"time":             time.Now().Format(time.RFC3339),
	})
}

// Start begins the gateway server
func (g *StarlinkGateway) Start() error {
	http.HandleFunc("/proxy", g.handleProxyRequest)
	http.HandleFunc("/register", g.handleNodeRegistration)
	http.HandleFunc("/health", g.healthCheck)

	addr := fmt.Sprintf(":%d", g.config.ListenPort)
	log.Printf("Starlink Gateway starting on %s", addr)
	log.Printf("Traffic mixing: %v", g.config.Anonymization.TrafficMixing)
	log.Printf("Authenticated nodes: %v", g.config.AuthenticatedNodes)
	
	return http.ListenAndServe(addr, nil)
}

// generateToken creates a random authentication token
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	configPath := "config/gateway.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	gateway, err := NewStarlinkGateway(configPath)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	if err := gateway.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
