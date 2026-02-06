package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/yourusername/proxy-system/common"
	"gopkg.in/yaml.v3"
)

// CentralConfig configuration for central proxy
type CentralConfig struct {
	ListenPort          int                     `yaml:"listen_port"`
	DownstreamServers   []string                `yaml:"downstream_servers"`
	ReassemblyTimeout   int                     `yaml:"reassembly_timeout"` // milliseconds
	ProxyMode           string                  `yaml:"proxy_mode"`         // "http" or "socks5"
	Encryption          common.EncryptionConfig `yaml:"encryption"`
	EncryptionKey       []byte                  `yaml:"-"`
	ChunkSize           int                     `yaml:"chunk_size"` // for response fragmentation
}

// CentralProxy aggregates chunks and performs actual proxying
type CentralProxy struct {
	config   CentralConfig
	sessions map[string]*common.Session
	mu       sync.RWMutex
	client   *http.Client
}

// NewCentralProxy creates a new central proxy instance
func NewCentralProxy(configPath string) (*CentralProxy, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config CentralConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.ChunkSize == 0 {
		config.ChunkSize = 8192
	}

	// Generate or load encryption key
	config.EncryptionKey = make([]byte, 32)
	copy(config.EncryptionKey, []byte("your-32-byte-encryption-key-here"))

	proxy := &CentralProxy{
		config:   config,
		sessions: make(map[string]*common.Session),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	// Start session cleanup goroutine
	go proxy.cleanupSessions()

	return proxy, nil
}

// handleChunk processes incoming chunk from upstream servers
func (p *CentralProxy) handleChunk(w http.ResponseWriter, r *http.Request) {
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

	// Decrypt if enabled
	if p.config.Encryption.Enabled {
		decrypted, err := common.DecryptAES(chunk.Data, p.config.EncryptionKey)
		if err != nil {
			http.Error(w, "Decryption failed", http.StatusInternalServerError)
			log.Printf("Decryption error: %v", err)
			return
		}
		chunk.Data = decrypted
	}

	log.Printf("Central received chunk %d/%d for session %s", 
		chunk.SequenceNum, chunk.TotalChunks, chunk.SessionID)

	// Add to session
	p.mu.Lock()
	session, exists := p.sessions[chunk.SessionID]
	if !exists {
		session = &common.Session{
			SessionID:   chunk.SessionID,
			Chunks:      make(map[int]*common.Chunk),
			TotalChunks: chunk.TotalChunks,
			ReceivedAt:  time.Now(),
			TargetURL:   chunk.TargetURL,
			Method:      chunk.Method,
			Headers:     chunk.Headers,
		}
		p.sessions[chunk.SessionID] = session
	}
	session.Chunks[chunk.SequenceNum] = chunk
	p.mu.Unlock()

	// Check if we have all chunks
	if len(session.Chunks) == session.TotalChunks {
		go p.processCompleteSession(session)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Chunk received"))
}

// processCompleteSession reassembles and proxies the request
func (p *CentralProxy) processCompleteSession(session *common.Session) {
	log.Printf("Session %s complete, reassembling and proxying", session.SessionID)

	// Reassemble chunks in order
	var fullData bytes.Buffer
	for i := 1; i <= session.TotalChunks; i++ {
		chunk, exists := session.Chunks[i]
		if !exists {
			log.Printf("Missing chunk %d for session %s", i, session.SessionID)
			return
		}
		fullData.Write(chunk.Data)
	}

	// Perform actual HTTP proxy request
	response, err := p.performProxyRequest(session, fullData.Bytes())
	if err != nil {
		log.Printf("Proxy request failed for session %s: %v", session.SessionID, err)
		return
	}

	// Fragment response and send to downstream servers
	if err := p.fragmentAndForward(session, response); err != nil {
		log.Printf("Failed to forward response for session %s: %v", session.SessionID, err)
	}

	// Cleanup session
	p.mu.Lock()
	delete(p.sessions, session.SessionID)
	p.mu.Unlock()
}

// performProxyRequest makes the actual HTTP request
func (p *CentralProxy) performProxyRequest(session *common.Session, body []byte) ([]byte, error) {
	req, err := http.NewRequest(session.Method, session.TargetURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}

	// Set headers from session
	for k, v := range session.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response read error: %w", err)
	}

	log.Printf("Proxied request to %s, received %d bytes", session.TargetURL, len(responseData))
	return responseData, nil
}

// fragmentAndForward splits response and sends to downstream servers
func (p *CentralProxy) fragmentAndForward(session *common.Session, response []byte) error {
	// Calculate number of chunks
	totalChunks := (len(response) + p.config.ChunkSize - 1) / p.config.ChunkSize
	
	log.Printf("Fragmenting response into %d chunks", totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * p.config.ChunkSize
		end := start + p.config.ChunkSize
		if end > len(response) {
			end = len(response)
		}

		chunk := &common.Chunk{
			SessionID:    session.SessionID,
			SequenceNum:  i + 1,
			TotalChunks:  totalChunks,
			Data:         response[start:end],
			Timestamp:    time.Now(),
			SourceClient: session.Chunks[1].SourceClient,
		}

		// Encrypt chunk if enabled
		if p.config.Encryption.Enabled {
			encrypted, err := common.EncryptAES(chunk.Data, p.config.EncryptionKey)
			if err != nil {
				return fmt.Errorf("encryption error: %w", err)
			}
			chunk.Data = encrypted
		}

		// Select downstream server (round-robin)
		downstreamURL := p.config.DownstreamServers[i%len(p.config.DownstreamServers)]
		
		if err := p.sendToDownstream(chunk, downstreamURL); err != nil {
			log.Printf("Failed to send chunk %d to %s: %v", i+1, downstreamURL, err)
		}
	}

	return nil
}

// sendToDownstream forwards chunk to downstream server
func (p *CentralProxy) sendToDownstream(chunk *common.Chunk, downstreamURL string) error {
	data, err := common.SerializeChunk(chunk)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/chunk", downstreamURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downstream returned status %d", resp.StatusCode)
	}

	return nil
}

// cleanupSessions removes expired sessions
func (p *CentralProxy) cleanupSessions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	timeout := time.Duration(p.config.ReassemblyTimeout) * time.Millisecond

	for range ticker.C {
		p.mu.Lock()
		now := time.Now()
		for sessionID, session := range p.sessions {
			if now.Sub(session.ReceivedAt) > timeout {
				log.Printf("Session %s timed out", sessionID)
				delete(p.sessions, sessionID)
			}
		}
		p.mu.Unlock()
	}
}

// healthCheck endpoint
func (p *CentralProxy) healthCheck(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	sessionCount := len(p.sessions)
	p.mu.RUnlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "healthy",
		"role":          "central-proxy",
		"active_sessions": sessionCount,
		"time":          time.Now().Format(time.RFC3339),
	})
}

// Start begins the central proxy server
func (p *CentralProxy) Start() error {
	http.HandleFunc("/chunk", p.handleChunk)
	http.HandleFunc("/health", p.healthCheck)

	addr := fmt.Sprintf(":%d", p.config.ListenPort)
	log.Printf("Central proxy starting on %s", addr)
	log.Printf("Downstream servers: %v", p.config.DownstreamServers)
	
	return http.ListenAndServe(addr, nil)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	
	configPath := "config/central.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	proxy, err := NewCentralProxy(configPath)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	if err := proxy.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
