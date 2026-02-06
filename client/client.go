package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dudelovecamera/proxy-system/common"
	"gopkg.in/yaml.v3"
)

// ClientConfig configuration for the client
type ClientConfig struct {
	ChunkSize       int      `yaml:"chunk_size"`
	UpstreamServers []string `yaml:"upstream_servers"`
	DownstreamPort  int      `yaml:"downstream_port"` // Port to listen for responses
	Timeout         int      `yaml:"timeout"`         // milliseconds
	Encryption      struct {
		Enabled   bool   `yaml:"enabled"`
		Algorithm string `yaml:"algorithm"`
		Mode      string `yaml:"mode"`
	} `yaml:"encryption"`
	EncryptionKey []byte `yaml:"-"`
}

// ProxyClient handles all client operations
type ProxyClient struct {
	config          ClientConfig
	pendingSessions map[string]*PendingSession
	mu              sync.RWMutex
	httpClient      *http.Client
	responseServer  *http.Server
}

// PendingSession tracks an outgoing request waiting for response
type PendingSession struct {
	SessionID    string
	RequestURL   string
	Method       string
	StartTime    time.Time
	ResponseChan chan *ProxyResponse
	Chunks       map[int]*common.Chunk
	TotalChunks  int
	mu           sync.Mutex
}

// ProxyResponse represents the final assembled response
type ProxyResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Error      error
}

// NewProxyClient creates a new client instance
func NewProxyClient(configPath string) (*ProxyClient, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config ClientConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.ChunkSize == 0 {
		config.ChunkSize = 8192
	}
	if config.DownstreamPort == 0 {
		config.DownstreamPort = 7000
	}
	if config.Timeout == 0 {
		config.Timeout = 30000
	}

	// Generate or load encryption key
	config.EncryptionKey = make([]byte, 32)
	copy(config.EncryptionKey, []byte("your-32-byte-encryption-key-here"))

	client := &ProxyClient{
		config:          config,
		pendingSessions: make(map[string]*PendingSession),
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Millisecond,
		},
	}

	return client, nil
}

// Start begins listening for downstream responses
func (c *ProxyClient) Start() error {
	// Start HTTP server to receive chunks from downstream servers
	mux := http.NewServeMux()
	mux.HandleFunc("/chunk", c.handleResponseChunk)
	mux.HandleFunc("/health", c.healthCheck)

	c.responseServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", c.config.DownstreamPort),
		Handler: mux,
	}

	log.Printf("Client listening for responses on port %d", c.config.DownstreamPort)

	return c.responseServer.ListenAndServe()
}

// MakeRequest sends a proxied HTTP request
func (c *ProxyClient) MakeRequest(method, url string, body []byte, headers map[string]string) (*ProxyResponse, error) {
	// Generate session ID
	sessionID := generateSessionID()

	log.Printf("Making request to %s (Session: %s)", url, sessionID)

	// Create pending session
	session := &PendingSession{
		SessionID:    sessionID,
		RequestURL:   url,
		Method:       method,
		StartTime:    time.Now(),
		ResponseChan: make(chan *ProxyResponse, 1),
		Chunks:       make(map[int]*common.Chunk),
	}

	c.mu.Lock()
	c.pendingSessions[sessionID] = session
	c.mu.Unlock()

	// Fragment and send request
	if err := c.fragmentAndSend(sessionID, method, url, body, headers); err != nil {
		c.mu.Lock()
		delete(c.pendingSessions, sessionID)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response or timeout
	timeout := time.Duration(c.config.Timeout) * time.Millisecond
	select {
	case response := <-session.ResponseChan:
		c.mu.Lock()
		delete(c.pendingSessions, sessionID)
		c.mu.Unlock()
		return response, response.Error

	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pendingSessions, sessionID)
		c.mu.Unlock()
		return nil, fmt.Errorf("request timeout after %v", timeout)
	}
}

// fragmentAndSend splits request into chunks and distributes to upstream servers
func (c *ProxyClient) fragmentAndSend(sessionID, method, url string, body []byte, headers map[string]string) error {
	// Calculate number of chunks
	totalChunks := (len(body) + c.config.ChunkSize - 1) / c.config.ChunkSize
	if totalChunks == 0 {
		totalChunks = 1 // At least one chunk even for empty body
	}

	log.Printf("Fragmenting request into %d chunks of ~%d bytes", totalChunks, c.config.ChunkSize)

	// Get client IP for downstream to send response back
	clientAddr := fmt.Sprintf("client:%d", c.config.DownstreamPort)

	for i := 0; i < totalChunks; i++ {
		start := i * c.config.ChunkSize
		end := start + c.config.ChunkSize
		if end > len(body) {
			end = len(body)
		}

		chunkData := body[start:end]
		if len(chunkData) == 0 && i == 0 {
			chunkData = []byte{} // Empty chunk for requests with no body
		}

		// Encrypt chunk if enabled
		if c.config.Encryption.Enabled {
			encrypted, err := common.EncryptAES(chunkData, c.config.EncryptionKey)
			if err != nil {
				return fmt.Errorf("encryption failed: %w", err)
			}
			chunkData = encrypted
		}

		chunk := &common.Chunk{
			SessionID:    sessionID,
			SequenceNum:  i + 1,
			TotalChunks:  totalChunks,
			Data:         chunkData,
			Timestamp:    time.Now(),
			SourceClient: clientAddr,
			TargetURL:    url,
			Method:       method,
			Headers:      headers,
		}

		// Select upstream server (round-robin)
		upstreamURL := c.config.UpstreamServers[i%len(c.config.UpstreamServers)]

		// Send chunk
		if err := c.sendChunk(chunk, upstreamURL); err != nil {
			log.Printf("Failed to send chunk %d to %s: %v", i+1, upstreamURL, err)
			// Continue sending other chunks
		} else {
			log.Printf("Sent chunk %d/%d to %s", i+1, totalChunks, upstreamURL)
		}
	}

	return nil
}

// sendChunk sends a single chunk to an upstream server
func (c *ProxyClient) sendChunk(chunk *common.Chunk, upstreamURL string) error {
	data, err := common.SerializeChunk(chunk)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/chunk", upstreamURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	return nil
}

// handleResponseChunk receives response chunks from downstream servers
func (c *ProxyClient) handleResponseChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	chunk, err := common.DeserializeChunk(body)
	if err != nil {
		http.Error(w, "Invalid chunk format", http.StatusBadRequest)
		return
	}

	// Decrypt chunk if enabled
	if c.config.Encryption.Enabled {
		decrypted, err := common.DecryptAES(chunk.Data, c.config.EncryptionKey)
		if err != nil {
			http.Error(w, "Decryption failed", http.StatusInternalServerError)
			log.Printf("Decryption error: %v", err)
			return
		}
		chunk.Data = decrypted
	}

	log.Printf("Received response chunk %d/%d for session %s",
		chunk.SequenceNum, chunk.TotalChunks, chunk.SessionID)

	// Find pending session
	c.mu.RLock()
	session, exists := c.pendingSessions[chunk.SessionID]
	c.mu.RUnlock()

	if !exists {
		log.Printf("No pending session found for %s", chunk.SessionID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Add chunk to session
	session.mu.Lock()
	session.Chunks[chunk.SequenceNum] = chunk
	session.TotalChunks = chunk.TotalChunks
	session.mu.Unlock()

	// Check if we have all chunks
	if len(session.Chunks) == session.TotalChunks {
		go c.assembleResponse(session)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Chunk received"))
}

// assembleResponse reassembles all chunks into final response
func (c *ProxyClient) assembleResponse(session *PendingSession) {
	session.mu.Lock()
	defer session.mu.Unlock()

	log.Printf("Assembling response for session %s (%d chunks)",
		session.SessionID, session.TotalChunks)

	// Reassemble chunks in order
	var fullResponse bytes.Buffer
	for i := 1; i <= session.TotalChunks; i++ {
		chunk, exists := session.Chunks[i]
		if !exists {
			session.ResponseChan <- &ProxyResponse{
				Error: fmt.Errorf("missing chunk %d", i),
			}
			return
		}
		fullResponse.Write(chunk.Data)
	}

	// Create response
	response := &ProxyResponse{
		StatusCode: 200, // In production, get this from response metadata
		Headers:    make(map[string]string),
		Body:       fullResponse.Bytes(),
		Error:      nil,
	}

	log.Printf("Response assembled: %d bytes", len(response.Body))

	// Send to waiting goroutine
	select {
	case session.ResponseChan <- response:
	default:
		log.Printf("Response channel full for session %s", session.SessionID)
	}
}

// healthCheck endpoint
func (c *ProxyClient) healthCheck(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	pendingCount := len(c.pendingSessions)
	c.mu.RUnlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "healthy",
		"role":             "proxy-client",
		"pending_sessions": pendingCount,
		"time":             time.Now().Format(time.RFC3339),
	})
}

// GET performs an HTTP GET request through the proxy
func (c *ProxyClient) GET(url string, headers map[string]string) (*ProxyResponse, error) {
	return c.MakeRequest("GET", url, nil, headers)
}

// POST performs an HTTP POST request through the proxy
func (c *ProxyClient) POST(url string, body []byte, headers map[string]string) (*ProxyResponse, error) {
	return c.MakeRequest("POST", url, body, headers)
}

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Example usage
func main() {
	configPath := "config/client.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	client, err := NewProxyClient(configPath)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Start listening for responses in background
	go func() {
		if err := client.Start(); err != nil {
			log.Fatalf("Client server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(1 * time.Second)

	log.Println("Proxy client ready!")
	log.Println("\nExample usage:")
	log.Println("  response, err := client.GET(\"http://example.com\", nil)")
	log.Println("  response, err := client.POST(\"http://api.example.com/data\", body, headers)")

	// Example request (commented out - uncomment to test)
	/*
		headers := map[string]string{
			"User-Agent": "ProxyClient/1.0",
			"Accept": "text/html",
		}

		response, err := client.GET("http://example.com", headers)
		if err != nil {
			log.Printf("Request failed: %v", err)
		} else {
			log.Printf("Response received: %d bytes", len(response.Body))
			log.Printf("First 100 chars: %s", string(response.Body[:min(100, len(response.Body))]))
		}
	*/

	// Keep running
	select {}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
