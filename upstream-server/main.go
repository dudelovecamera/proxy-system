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

	"github.com/dudelovecamera/proxy-system/common"
	"gopkg.in/yaml.v3"
)

// UpstreamConfig configuration for upstream server
type UpstreamConfig struct {
	ListenPort    int                      `yaml:"listen_port"`
	CentralProxy  string                   `yaml:"central_proxy"`
	Obfuscation   common.ObfuscationConfig `yaml:"obfuscation"`
	Encryption    common.EncryptionConfig  `yaml:"encryption"`
	EncryptionKey []byte                   `yaml:"-"` // 32 bytes for AES-256
}

// UpstreamServer handles incoming chunks from clients
type UpstreamServer struct {
	config UpstreamConfig
	client *http.Client
	mu     sync.RWMutex
}

// NewUpstreamServer creates a new upstream server instance
func NewUpstreamServer(configPath string) (*UpstreamServer, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config UpstreamConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Generate or load encryption key (in production, use secure key management)
	config.EncryptionKey = make([]byte, 32)
	// For demo: use a fixed key. In production: load from secure storage
	copy(config.EncryptionKey, []byte("your-32-byte-encryption-key-here"))

	return &UpstreamServer{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// handleChunk processes incoming chunk from client
func (s *UpstreamServer) handleChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read chunk data
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		log.Printf("Error reading body: %v", err)
		return
	}
	defer r.Body.Close()

	// Deserialize chunk
	chunk, err := common.DeserializeChunk(body)
	if err != nil {
		http.Error(w, "Invalid chunk format", http.StatusBadRequest)
		log.Printf("Error deserializing chunk: %v", err)
		return
	}

	log.Printf("Received chunk %d/%d for session %s",
		chunk.SequenceNum, chunk.TotalChunks, chunk.SessionID)

	// Apply obfuscation
	chunk.Headers = common.ApplyObfuscation(chunk.Headers, s.config.Obfuscation)

	// Apply encryption if enabled
	if s.config.Encryption.Enabled {
		encrypted, err := common.EncryptAES(chunk.Data, s.config.EncryptionKey)
		if err != nil {
			http.Error(w, "Encryption failed", http.StatusInternalServerError)
			log.Printf("Encryption error: %v", err)
			return
		}
		chunk.Data = encrypted
	}

	// Add timing jitter if configured
	if s.config.Obfuscation.Jitter > 0 {
		jitter := time.Duration(s.config.Obfuscation.Jitter) * time.Millisecond
		time.Sleep(jitter)
	}

	// Forward to central proxy
	if err := s.forwardToCentral(chunk); err != nil {
		http.Error(w, "Failed to forward chunk", http.StatusInternalServerError)
		log.Printf("Forwarding error: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Chunk received and forwarded"))
}

// forwardToCentral sends chunk to central proxy server
func (s *UpstreamServer) forwardToCentral(chunk *common.Chunk) error {
	data, err := common.SerializeChunk(chunk)
	if err != nil {
		return fmt.Errorf("serialization error: %w", err)
	}

	url := fmt.Sprintf("http://%s/chunk", s.config.CentralProxy)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	// Set obfuscation headers
	for k, v := range s.config.Obfuscation.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("central proxy returned status %d", resp.StatusCode)
	}

	return nil
}

// healthCheck endpoint for monitoring
func (s *UpstreamServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"role":   "upstream",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Start begins listening for incoming chunks
func (s *UpstreamServer) Start() error {
	http.HandleFunc("/chunk", s.handleChunk)
	http.HandleFunc("/health", s.healthCheck)

	addr := fmt.Sprintf(":%d", s.config.ListenPort)
	log.Printf("Upstream server starting on %s", addr)
	log.Printf("Forwarding to central proxy: %s", s.config.CentralProxy)

	return http.ListenAndServe(addr, nil)
}

func main() {
	configPath := "config/upstream.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	server, err := NewUpstreamServer(configPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
