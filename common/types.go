package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"time"
	rando "math/rand"
)

// Chunk represents a fragmented packet
type Chunk struct {
	SessionID    string    `json:"session_id"`
	SequenceNum  int       `json:"sequence_num"`
	TotalChunks  int       `json:"total_chunks"`
	Data         []byte    `json:"data"`
	Timestamp    time.Time `json:"timestamp"`
	SourceClient string    `json:"source_client"`
	TargetURL    string    `json:"target_url"`
	Method       string    `json:"method"`
	Headers      map[string]string `json:"headers"`
}

// ObfuscationConfig defines obfuscation settings
type ObfuscationConfig struct {
	Type    string            `yaml:"type" json:"type"`
	Headers map[string]string `yaml:"headers" json:"headers"`
	Padding bool              `yaml:"padding" json:"padding"`
	Jitter  int               `yaml:"jitter" json:"jitter"` // milliseconds
}

// EncryptionConfig defines encryption settings
type EncryptionConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Algorithm string `yaml:"algorithm" json:"algorithm"`
	Mode      string `yaml:"mode" json:"mode"` // "body_only" or "full_request"
}

// ServerConfig common server configuration
type ServerConfig struct {
	ListenPort   int                 `yaml:"listen_port" json:"listen_port"`
	Obfuscation  ObfuscationConfig   `yaml:"obfuscation" json:"obfuscation"`
	Encryption   EncryptionConfig    `yaml:"encryption" json:"encryption"`
	Timeout      int                 `yaml:"timeout" json:"timeout"` // milliseconds
}

// Session tracks reassembly state
type Session struct {
	SessionID   string
	Chunks      map[int]*Chunk
	TotalChunks int
	ReceivedAt  time.Time
	TargetURL   string
	Method      string
	Headers     map[string]string
}

// EncryptAES encrypts data using AES-256-GCM
func EncryptAES(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptAES decrypts data using AES-256-GCM
func DecryptAES(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GenerateSessionID creates a unique session identifier
func GenerateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

// SerializeChunk converts chunk to JSON
func SerializeChunk(chunk *Chunk) ([]byte, error) {
	return json.Marshal(chunk)
}

// DeserializeChunk converts JSON to chunk
func DeserializeChunk(data []byte) (*Chunk, error) {
	var chunk Chunk
	err := json.Unmarshal(data, &chunk)
	return &chunk, err
}

// ApplyObfuscation adds obfuscation headers
func ApplyObfuscation(headers map[string]string, config ObfuscationConfig) map[string]string {
	obfuscated := make(map[string]string)
	
	// Copy original headers
	for k, v := range headers {
		obfuscated[k] = v
	}
	
	// Add obfuscation headers
	for k, v := range config.Headers {
		obfuscated[k] = v
	}
	
	return obfuscated
}

// AddRandomPadding adds random padding to data
func AddRandomPadding(data []byte, minPadding, maxPadding int) []byte {
	paddingSize := minPadding
	if maxPadding > minPadding {
		paddingSize += int(rando.Int31n(int32(maxPadding - minPadding)))
	}
	
	padding := make([]byte, paddingSize)
	rand.Read(padding)
	
	return append(data, padding...)
}
