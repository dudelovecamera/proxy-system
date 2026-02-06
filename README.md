# Distributed Multi-Path Proxy System

A sophisticated distributed proxy system with packet-level fragmentation, multi-path routing, encryption, and traffic obfuscation capabilities.

## Architecture Overview

```
Client
  ↓
[Upstream Servers] (3+) - Fragment & Encrypt
  ↓
[Central Proxy] (1-2) - Reassemble & Proxy
  ↓
[Downstream Servers] (3+) - Fragment Responses
  ↓
[Relay Nodes] (2+) - Isolation Layer
  ↓
[Starlink Gateway] (1) - Internet Access with Anonymization
  ↓
Internet
```

## Server Requirements

### Minimum Setup (7 servers):
- **2-3 Upstream Servers** - Entry points for client traffic
- **1 Central Proxy Server** - Aggregator and actual proxy
- **2-3 Downstream Servers** - Response delivery
- **1 Starlink Gateway** - Internet gateway

### Recommended Production Setup (10-15 servers):
- **3-5 Upstream Servers**
- **1-2 Central Proxy Servers** (for redundancy)
- **3-5 Downstream Servers**
- **2-4 Relay Nodes** (gateway isolation layer)
- **1 Starlink Gateway**

## Features

### Traffic Management
- ✅ Packet-level fragmentation (configurable chunk size)
- ✅ Multi-path routing across servers
- ✅ Session management with timeout handling
- ✅ Automatic reassembly with ordering

### Security & Anonymization
- ✅ AES-256-GCM encryption
- ✅ HTTP header obfuscation (mimic legitimate traffic)
- ✅ Timing randomization (jitter)
- ✅ Traffic mixing (batch multiple requests)
- ✅ Gateway IP isolation (multi-hop relay architecture)

### Operational Features
- ✅ Health check endpoints for monitoring
- ✅ Dynamic route rotation
- ✅ Node authentication and authorization
- ✅ Configurable via YAML
- ✅ Docker deployment ready

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+ (for local development)

### 1. Clone and Setup

```bash
git clone <repository>
cd proxy-system
```

### 2. Configure Servers

Edit configuration files in `config/` directory:
- `upstream.yaml` - Upstream server settings
- `central.yaml` - Central proxy settings
- `downstream.yaml` - Downstream server settings
- `relay.yaml` - Relay node settings
- `gateway.yaml` - Starlink gateway settings

### 3. Deploy with Docker Compose

```bash
# Build and start all services
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

### 4. Verify Deployment

```bash
# Check upstream server
curl http://localhost:8001/health

# Check central proxy
curl http://localhost:8080/health

# Check downstream server
curl http://localhost:8443/health

# Check relay node
curl http://localhost:8500/health

# Check gateway
curl http://localhost:9000/health
```

## Configuration Guide

### Upstream Server (`config/upstream.yaml`)

```yaml
listen_port: 8001
central_proxy: "central-proxy:8080"

obfuscation:
  type: "http_mimic"
  headers:
    User-Agent: "Mozilla/5.0..."
    Accept-Language: "en-US,en;q=0.9"
  padding: true
  jitter: 100  # milliseconds

encryption:
  enabled: true
  algorithm: "aes-256-gcm"
  mode: "body_only"
```

### Central Proxy (`config/central.yaml`)

```yaml
listen_port: 8080
downstream_servers:
  - "downstream1:8443"
  - "downstream2:8444"
  - "downstream3:8445"
reassembly_timeout: 60000  # ms
chunk_size: 8192  # bytes
```

### Gateway (`config/gateway.yaml`)

```yaml
listen_port: 9000
authenticated_nodes:
  - "relay1.internal"
  - "relay2.internal"

anonymization:
  traffic_mixing: true
  source_rotation: true
  timing_jitter: 500
```

## Production Deployment

### 1. Generate Encryption Keys

```bash
# Generate 32-byte AES key
openssl rand -hex 32

# Update all config files with the same key
# Or use environment variables
```

### 2. TLS/SSL Configuration

For production, enable HTTPS:
- Obtain SSL certificates (Let's Encrypt recommended)
- Configure reverse proxy (Nginx/Caddy)
- Update server configurations

### 3. Firewall Rules

```bash
# Upstream servers - only accept from clients
ufw allow 8001:8003/tcp

# Central proxy - only accept from upstream
ufw allow from <upstream-ips> to any port 8080

# Downstream - only accept from central
ufw allow from <central-ip> to any port 8443:8445

# Gateway - only accept from relays
ufw allow from <relay-ips> to any port 9000
```

### 4. Monitoring Setup

Use Prometheus + Grafana:

```bash
# Add to docker-compose.yml
prometheus:
  image: prom/prometheus
  volumes:
    - ./prometheus.yml:/etc/prometheus/prometheus.yml

grafana:
  image: grafana/grafana
  ports:
    - "3000:3000"
```

## Development

### Local Build

```bash
# Build upstream server
cd upstream-server
go build -o upstream main.go
./upstream ../config/upstream.yaml

# Build central proxy
cd central-proxy
go build -o central main.go
./central ../config/central.yaml

# Similar for other servers...
```

### Run Tests

```bash
# Unit tests
go test ./...

# Integration tests
./scripts/integration-test.sh
```

## Traffic Flow Example

1. **Client Request**:
   - Client sends HTTP request
   - Fragmented into 3 chunks (8KB each)
   - Sent to 3 different upstream servers

2. **Upstream Processing**:
   - Each server receives 1 chunk
   - Applies encryption (AES-256-GCM)
   - Adds obfuscation headers
   - Forwards to central proxy

3. **Central Proxy**:
   - Receives all 3 chunks
   - Reassembles original request
   - Performs actual HTTP proxy
   - Fragments response
   - Distributes to downstream servers

4. **Downstream Delivery**:
   - Each server receives response chunk
   - Applies obfuscation
   - Forwards to relay nodes

5. **Relay & Gateway**:
   - Relay nodes mix traffic
   - Final relay forwards to gateway
   - Gateway performs traffic anonymization
   - Sends to internet

## Security Considerations

### Encryption Key Management
- Use environment variables or secure vaults
- Rotate keys periodically
- Never commit keys to version control

### Network Isolation
- Use VLANs or separate networks
- Implement strict firewall rules
- Enable fail2ban for brute force protection

### Logging
- Minimize logs (privacy)
- No traffic content logging
- Rotate logs frequently
- Consider log encryption

### Authentication
- Strong tokens for node authentication
- Regular token rotation
- Monitor for unauthorized access

## Performance Tuning

### Chunk Size
- Smaller chunks = more overhead, better distribution
- Larger chunks = less overhead, fewer packets
- Recommended: 4KB - 16KB

### Timeout Settings
- Adjust based on network latency
- Higher timeouts = more memory usage
- Lower timeouts = more retries

### Concurrency
- Increase worker goroutines for high traffic
- Monitor memory usage
- Use connection pooling

## Troubleshooting

### Chunks Not Reassembling
```bash
# Check logs
docker-compose logs central-proxy | grep "Session.*complete"

# Verify all upstream servers are sending
docker-compose logs upstream1 | grep "Chunk received"

# Check timeout settings
```

### High Latency
```bash
# Disable traffic mixing temporarily
# Reduce jitter values
# Increase server resources
```

### Authentication Failures
```bash
# Verify tokens in gateway logs
docker-compose logs gateway | grep "Authentication"

# Re-register relay nodes
curl -X POST http://gateway:9000/register \
  -d '{"node_id":"relay1","secret":"secret"}'
```

## Roadmap

- [ ] Client application (CLI and GUI)
- [ ] Android/iOS mobile apps
- [ ] SOCKS5 proxy mode
- [ ] WebSocket support
- [ ] ML-based traffic shaping
- [ ] Blockchain-based server discovery
- [ ] Zero-knowledge authentication

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create feature branch
3. Add tests
4. Submit pull request

## License

MIT License - see LICENSE file

## Support

For issues and questions:
- GitHub Issues: <repository-url>/issues
- Documentation: <docs-url>
- Community: <discord/slack-url>

---

**⚠️ Disclaimer**: This software is for educational and authorized security testing purposes only. Users are responsible for compliance with applicable laws and regulations.
