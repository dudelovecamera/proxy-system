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

// DownstreamConfig configuration for downstream server
type DownstreamConfig struct {
	ListenPort        int                      `yaml:"listen_port"`
	Obfuscation       common.ObfuscationConfig `yaml:"obfuscation"`
	Encryption        common.EncryptionConfig  `yaml:"encryption"`
	EncryptionKey     []byte                   `yaml:"-"`
	ReassemblyTimeout int                      `yaml:"reassembly_timeout"` // milliseconds
}

// DownstreamServer handles response chunks and delivers to clients
type DownstreamServer struct {
	config   DownstreamConfig
	sessions map[string]*common.Session
	mu       sync.RWMutex
	client   *http.Client
}

// NewDownstreamServer creates a new downstream server instance
func NewDownstreamServer(configPath string) (*DownstreamServer, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config DownstreamConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if config.ReassemblyTimeout == 0 {
		config.ReassemblyTimeout = 60000 // 60 seconds default
	}

	// Generate or load encryption key
	config.EncryptionKey = make([]byte, 32)
	copy(config.EncryptionKey, []byte("your-32-byte-encryption-key-here"))

	server := &DownstreamServer{
		config:   config,
		sessions: make(map[string]*common.Session),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Start session cleanup
	go server.cleanupSessions()

	return server, nil
}

// handleChunk processes incoming response chunk from central proxy
func (s *DownstreamServer) handleChunk(w http.ResponseWriter, r *http.Request) {
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
	if s.config.Encryption.Enabled {
		decrypted, err := common.DecryptAES(chunk.Data, s.config.EncryptionKey)
		if err != nil {
			http.Error(w, "Decryption failed", http.StatusInternalServerError)
			log.Printf("Decryption error: %v", err)
			return
		}
		chunk.Data = decrypted
	}

	log.Printf("Downstream received chunk %d/%d for session %s",
		chunk.SequenceNum, chunk.TotalChunks, chunk.SessionID)

	// Add to session
	s.mu.Lock()
	session, exists := s.sessions[chunk.SessionID]
	if !exists {
		session = &common.Session{
			SessionID:   chunk.SessionID,
			Chunks:      make(map[int]*common.Chunk),
			TotalChunks: chunk.TotalChunks,
			ReceivedAt:  time.Now(),
		}
		s.sessions[chunk.SessionID] = session
	}
	session.Chunks[chunk.SequenceNum] = chunk
	s.mu.Unlock()

	// Check if we have all chunks
	if len(session.Chunks) == session.TotalChunks {
		go s.deliverToClient(session)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Chunk received"))
}

// deliverToClient reassembles response and sends to client
func (s *DownstreamServer) deliverToClient(session *common.Session) {
	log.Printf("Session %s complete, delivering to client", session.SessionID)

	// Get client address from first chunk
	clientAddr := session.Chunks[1].SourceClient
	if clientAddr == "" {
		log.Printf("No client address for session %s", session.SessionID)
		return
	}

	// Send each chunk back to client
	for i := 1; i <= session.TotalChunks; i++ {
		chunk, exists := session.Chunks[i]
		if !exists {
			log.Printf("Missing chunk %d for session %s", i, session.SessionID)
			continue
		}

		// Apply obfuscation if configured
		if s.config.Obfuscation.Type != "" {
			chunk.Headers = common.ApplyObfuscation(chunk.Headers, s.config.Obfuscation)
		}

		// Re-encrypt for client if needed
		if s.config.Encryption.Enabled {
			encrypted, err := common.EncryptAES(chunk.Data, s.config.EncryptionKey)
			if err != nil {
				log.Printf("Encryption error: %v", err)
				continue
			}
			chunk.Data = encrypted
		}

		// Send chunk to client
		if err := s.sendChunkToClient(chunk, clientAddr); err != nil {
			log.Printf("Failed to send chunk %d to client: %v", i, err)
		}
	}

	log.Printf("All %d chunks sent back to client %s", session.TotalChunks, clientAddr)

	// Cleanup session
	s.mu.Lock()
	delete(s.sessions, session.SessionID)
	s.mu.Unlock()
}

// sendChunkToClient sends a response chunk back to the client
func (s *DownstreamServer) sendChunkToClient(chunk *common.Chunk, clientAddr string) error {
	data, err := common.SerializeChunk(chunk)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/chunk", clientAddr)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client returned status %d", resp.StatusCode)
	}

	log.Printf("Sent response chunk %d/%d to client", chunk.SequenceNum, chunk.TotalChunks)
	return nil
}

// handleClientPoll allows clients to retrieve assembled responses
func (s *DownstreamServer) handleClientPoll(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id parameter", http.StatusBadRequest)
		return
	}

	// In production, retrieve from cache/queue
	// For now, return a placeholder
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Response data would be here"))
}

// cleanupSessions removes expired sessions
func (s *DownstreamServer) cleanupSessions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	timeout := time.Duration(s.config.ReassemblyTimeout) * time.Millisecond

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for sessionID, session := range s.sessions {
			if now.Sub(session.ReceivedAt) > timeout {
				log.Printf("Session %s timed out", sessionID)
				delete(s.sessions, sessionID)
			}
		}
		s.mu.Unlock()
	}
}

// healthCheck endpoint
func (s *DownstreamServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	sessionCount := len(s.sessions)
	s.mu.RUnlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "healthy",
		"role":            "downstream",
		"active_sessions": sessionCount,
		"time":            time.Now().Format(time.RFC3339),
	})
}

// Start begins the downstream server
func (s *DownstreamServer) Start() error {
	http.HandleFunc("/chunk", s.handleChunk)
	http.HandleFunc("/poll", s.handleClientPoll)
	http.HandleFunc("/health", s.healthCheck)

	addr := fmt.Sprintf(":%d", s.config.ListenPort)
	log.Printf("Downstream server starting on %s", addr)

	return http.ListenAndServe(addr, nil)
}

func main() {
	configPath := "config/downstream.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	server, err := NewDownstreamServer(configPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
