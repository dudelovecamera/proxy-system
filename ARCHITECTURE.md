# System Architecture

## Network Topology

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENT LAYER                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Client 1 │  │ Client 2 │  │ Client 3 │  │ Client N │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
└───────┼─────────────┼─────────────┼─────────────┼───────────────┘
        │             │             │             │
        │    Fragmented & Encrypted Packets       │
        ▼             ▼             ▼             ▼
┌─────────────────────────────────────────────────────────────────┐
│                     UPSTREAM SERVERS (Entry)                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │Upstream 1│  │Upstream 2│  │Upstream 3│  │Upstream N│        │
│  │ :8001    │  │ :8002    │  │ :8003    │  │ :800N    │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│       │             │             │             │                │
│  • Chunk Reception                                               │
│  • Encryption (AES-256-GCM)                                      │
│  • HTTP Obfuscation                                              │
│  • Timing Jitter                                                 │
└───────┼─────────────┼─────────────┼─────────────┼───────────────┘
        │             │             │             │
        └─────────────┴─────────────┴─────────────┘
                          │
        ┌─────────────────▼─────────────────┐
        │                                    │
┌───────▼───────────────────────────────────▼────────────────────┐
│                   CENTRAL PROXY SERVER                          │
│            ┌────────────────────────────┐                       │
│            │  Session Management        │                       │
│            │  • Chunk Aggregation       │                       │
│            │  • Reassembly Engine       │                       │
│            │  • Timeout Handling        │                       │
│            └────────────┬───────────────┘                       │
│                         │                                        │
│            ┌────────────▼───────────────┐                       │
│            │   Proxy Engine (:8080)     │                       │
│            │  • HTTP/HTTPS Requests     │                       │
│            │  • Response Fragmentation  │                       │
│            └────────────┬───────────────┘                       │
└─────────────────────────┼──────────────────────────────────────┘
                          │
        ┌─────────────────┴─────────────────┐
        │             │             │             │
┌───────▼─────────────▼─────────────▼─────────────▼───────────────┐
│                  DOWNSTREAM SERVERS (Exit)                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │Downstm 1 │  │Downstm 2 │  │Downstm 3 │  │Downstm N │        │
│  │ :8443    │  │ :8444    │  │ :8445    │  │ :844N    │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│       │             │             │             │                │
│  • Response Reassembly                                           │
│  • Encryption                                                    │
│  • HTTP Obfuscation                                              │
└───────┼─────────────┼─────────────┼─────────────┼───────────────┘
        │             │             │             │
        └─────────────┴─────────────┴─────────────┘
                          │
        ┌─────────────────▼─────────────────┐
        │             │             │             │
┌───────▼─────────────▼─────────────▼─────────────▼───────────────┐
│                    RELAY NODES (Isolation)                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Relay 1  │→ │ Relay 2  │→ │ Relay 3  │→ │ Relay N  │        │
│  │ :8500    │  │ :8501    │  │ :8502    │  │ :850N    │        │
│  └──────────┘  └──────────┘  └──────────┘  └────┬─────┘        │
│                                                   │               │
│  • Multi-hop Routing                             │               │
│  • Traffic Mixing                                │               │
│  • Gateway IP Isolation                          │               │
│  • Dynamic Path Rotation                         │               │
└──────────────────────────────────────────────────┼───────────────┘
                                                    │
                          ┌─────────────────────────▼──────┐
                          │                                 │
┌─────────────────────────▼─────────────────────────────────▼──────┐
│                   STARLINK GATEWAY (:9000)                        │
│  ┌────────────────────────────────────────────────────────┐      │
│  │  Internet Gateway & Anonymization Layer                │      │
│  │  • Traffic Aggregation & Mixing                        │      │
│  │  • Source IP Rotation (if multiple interfaces)         │      │
│  │  • MAC Randomization (requires root)                   │      │
│  │  • Timing Obfuscation                                  │      │
│  │  • Node Authentication                                 │      │
│  │  • Complete IP Isolation (no node knows gateway IP)    │      │
│  └────────────────────────┬───────────────────────────────┘      │
└───────────────────────────┼──────────────────────────────────────┘
                            │
                            ▼
                    ┌───────────────┐
                    │   INTERNET    │
                    └───────────────┘
```

## Data Flow Sequence

### Request Flow (Client → Internet)

1. **Client Fragmentation**
   ```
   HTTP Request (24KB)
   ├─ Chunk 1 (8KB) → Upstream Server 1
   ├─ Chunk 2 (8KB) → Upstream Server 2
   └─ Chunk 3 (8KB) → Upstream Server 3
   ```

2. **Upstream Processing**
   ```
   Each Chunk:
   ├─ Encrypt with AES-256-GCM
   ├─ Add obfuscation headers
   ├─ Apply timing jitter (0-500ms)
   └─ Forward to Central Proxy
   ```

3. **Central Aggregation**
   ```
   Central Proxy:
   ├─ Receive chunks from all upstreams
   ├─ Decrypt chunks
   ├─ Reassemble in sequence order
   ├─ Perform actual HTTP request
   └─ Fragment response
   ```

4. **Response Distribution**
   ```
   HTTP Response (48KB)
   ├─ Chunk 1 (8KB) → Downstream 1
   ├─ Chunk 2 (8KB) → Downstream 2
   ├─ Chunk 3 (8KB) → Downstream 3
   ├─ Chunk 4 (8KB) → Downstream 1
   ├─ Chunk 5 (8KB) → Downstream 2
   └─ Chunk 6 (8KB) → Downstream 3
   ```

5. **Relay Chain**
   ```
   Traffic Flow:
   Downstream → Relay 1 → Relay 2 → Relay 3 → Gateway
   
   At each relay:
   ├─ Traffic mixing (batch multiple requests)
   ├─ Path rotation
   └─ Forward to next hop
   ```

6. **Gateway Anonymization**
   ```
   Gateway:
   ├─ Aggregate traffic from all relays
   ├─ Mix traffic (batch processing)
   ├─ Rotate source IPs (if available)
   ├─ Apply timing jitter
   └─ Send to internet
   ```

## Security Layers

```
┌─────────────────────────────────────────────────┐
│ Layer 1: Encryption                             │
│ ├─ End-to-end AES-256-GCM                       │
│ └─ Per-hop re-encryption                        │
├─────────────────────────────────────────────────┤
│ Layer 2: Obfuscation                            │
│ ├─ HTTP header mimicry                          │
│ ├─ Legitimate user agents                       │
│ └─ Fake cookies & tracking                      │
├─────────────────────────────────────────────────┤
│ Layer 3: Traffic Pattern                        │
│ ├─ Chunk fragmentation                          │
│ ├─ Multi-path routing                           │
│ └─ Timing randomization                         │
├─────────────────────────────────────────────────┤
│ Layer 4: Network Isolation                      │
│ ├─ Multi-hop relay chains                       │
│ ├─ No single point knows full path              │
│ └─ Gateway IP never exposed                     │
├─────────────────────────────────────────────────┤
│ Layer 5: Traffic Mixing                         │
│ ├─ Batch processing                             │
│ ├─ Aggregation across nodes                     │
│ └─ Source rotation                              │
└─────────────────────────────────────────────────┘
```

## Deployment Configurations

### Minimal Setup (7 Servers)
- 2 Upstream, 1 Central, 2 Downstream, 1 Gateway, 1 Relay
- Best for: Testing, development, low traffic

### Standard Setup (10 Servers)
- 3 Upstream, 1 Central, 3 Downstream, 1 Gateway, 2 Relays
- Best for: Small production, moderate traffic

### High Availability (15 Servers)
- 5 Upstream, 2 Central, 5 Downstream, 1 Gateway, 2 Relays
- Best for: Production, high traffic, redundancy

### Maximum Anonymity (20+ Servers)
- 5 Upstream, 2 Central, 5 Downstream, 1 Gateway, 4+ Relays
- Best for: Maximum privacy, complex routing

## Port Mapping

| Service          | Internal Port | External Port | Protocol |
|------------------|---------------|---------------|----------|
| Upstream-1       | 8001          | 8001          | HTTP     |
| Upstream-2       | 8001          | 8002          | HTTP     |
| Upstream-3       | 8001          | 8003          | HTTP     |
| Central Proxy    | 8080          | 8080          | HTTP     |
| Downstream-1     | 8443          | 8443          | HTTP     |
| Downstream-2     | 8443          | 8444          | HTTP     |
| Downstream-3     | 8443          | 8445          | HTTP     |
| Relay-1          | 8500          | 8500          | HTTP     |
| Relay-2          | 8500          | 8501          | HTTP     |
| Gateway          | 9000          | 9000          | HTTP     |

## Resource Requirements

### Per Server Type

**Upstream Server:**
- CPU: 1-2 cores
- RAM: 512MB - 1GB
- Disk: 10GB
- Network: 100Mbps+

**Central Proxy:**
- CPU: 2-4 cores
- RAM: 2-4GB
- Disk: 20GB
- Network: 1Gbps+

**Downstream Server:**
- CPU: 1-2 cores
- RAM: 512MB - 1GB
- Disk: 10GB
- Network: 100Mbps+

**Relay Node:**
- CPU: 1 core
- RAM: 256MB - 512MB
- Disk: 5GB
- Network: 100Mbps+

**Gateway:**
- CPU: 2-4 cores
- RAM: 1-2GB
- Disk: 10GB
- Network: 1Gbps+ (critical)
