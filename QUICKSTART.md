# Quick Start - Windows Client Setup

## What You Got

✅ **Complete client applications** (CLI, GUI, and library)
✅ **Fixed downstream communication** (now sends back to client correctly)
✅ **Relay/Gateway explained** (similar to Tor, optional for anonymity)
✅ **Windows installation guide**
✅ **Code examples** for integration

---

## Understanding the Flow (Your Questions Answered!)

### ✅ CORRECTED Architecture

```
CLIENT (Windows PC)
  ↓ Fragment request into chunks
UPSTREAM SERVERS (2-3 servers)
  ↓ Encrypt & forward chunks
CENTRAL PROXY (1 server)
  ↓ Reassemble, make HTTP request
  ├─→ INTERNET (gets response)
  ↓ Fragment response into chunks
DOWNSTREAM SERVERS (2-3 servers)
  ↓ Send chunks DIRECTLY BACK TO CLIENT ✓
CLIENT (Windows PC)
  ↓ Reassemble response
DONE!
```

**Key Point:** Downstream talks to CLIENT, NOT gateway!

### Relay/Gateway (OPTIONAL - Like Tor)

Only needed if you want:
- **Hide gateway IP** from all other servers
- **Share one internet connection** (Starlink) across multiple proxy systems
- **Maximum anonymity** through multi-hop routing

```
If using relay/gateway:

CENTRAL PROXY
  ↓ (request only)
RELAY-1 → RELAY-2 → GATEWAY → INTERNET
  ↑ (response comes back same path)
CENTRAL PROXY
  ↓ (response to downstream)
DOWNSTREAM → CLIENT ✓
```

**Yes, it's like Tor!** Paths rotate every 5 minutes, no single relay knows the full route.

---

## Windows Installation (5 Minutes)

### Step 1: Extract Files
```
Right-click distributed-proxy-system-v2.tar.gz
→ Extract with 7-Zip
→ cd distributed-proxy-system
```

### Step 2: Install Go
- Download: https://go.dev/dl/
- Install `go1.21.windows-amd64.msi`
- Restart Command Prompt

### Step 3: Build Client
```cmd
build-windows.bat
```

This creates:
- `build/proxy-cli.exe` - Command-line client ✓
- `build/proxy-gui.exe` - GUI client ✓
- `build/config/client.yaml` - Configuration

### Step 4: Configure
Edit `build/config/client.yaml`:
```yaml
upstream_servers:
  - "your-server1.com:8001"  # ← Change this!
  - "your-server2.com:8002"  # ← Change this!
  - "your-server3.com:8003"  # ← Change this!
```

### Step 5: Test
```cmd
cd build
proxy-cli.exe -url http://example.com
```

---

## Usage Examples

### Command Line

**Simple GET:**
```cmd
proxy-cli.exe -url http://example.com
```

**POST with data:**
```cmd
proxy-cli.exe -method POST -url http://api.example.com/data -data "{\"test\":\"data\"}"
```

**Interactive mode:**
```cmd
proxy-cli.exe -i
```

### GUI Client

Double-click `proxy-gui.exe`:
1. Enter URL
2. Select method (GET/POST)
3. Enter body (if POST)
4. Click "Send Request"

### In Your Code

```go
import "github.com/dudelovecamera/proxy-system/client"

proxyClient, _ := client.NewProxyClient("config/client.yaml")
go proxyClient.Start()

response, err := proxyClient.GET("http://example.com", nil)
fmt.Println(string(response.Body))
```

---

## Server Requirements

### Minimum Setup (7 servers):
- 2 Upstream
- 1 Central
- 2 Downstream
- 1 Gateway (optional)
- 1 Relay (optional)

### Without Gateway (5 servers):
- 2 Upstream
- 1 Central
- 2 Downstream
✓ Simple, direct internet access from central proxy

---

## How It Works

### 1. You make a request:
```go
response, err := client.GET("http://example.com", nil)
```

### 2. Client fragments it:
```
Original request (24KB)
├─→ Chunk 1 (8KB) → Upstream-1
├─→ Chunk 2 (8KB) → Upstream-2
└─→ Chunk 3 (8KB) → Upstream-3
```

### 3. Upstream servers encrypt & forward:
```
Upstream-1 → [encrypted chunk] → Central Proxy
Upstream-2 → [encrypted chunk] → Central Proxy
Upstream-3 → [encrypted chunk] → Central Proxy
```

### 4. Central reassembles & fetches:
```
Central: Receives all 3 chunks
       → Decrypts & reassembles
       → Makes HTTP GET to example.com
       → Gets 48KB response
       → Fragments into 6 chunks
```

### 5. Downstream sends back to YOU:
```
Downstream-1 → Chunk 1 → Your PC
Downstream-2 → Chunk 2 → Your PC
Downstream-3 → Chunk 3 → Your PC
Downstream-1 → Chunk 4 → Your PC
... etc
```

### 6. You get the response:
```go
fmt.Println(response.Body) // Complete response!
```

---

## Relay/Gateway Deep Dive

### When to Use:
- ✅ Sharing one Starlink connection
- ✅ Want maximum anonymity
- ✅ Hide gateway IP from everyone
- ✅ Need Tor-like protection

### When NOT to Use:
- ❌ Each server has its own internet
- ❌ Simple use case
- ❌ Need low latency
- ❌ Less than 5 total servers

### How Relay Rotation Works:

**Time 0:00:** Central → Relay-A → Relay-B → Gateway
**Time 5:00:** Central → Relay-C → Relay-D → Gateway
**Time 10:00:** Central → Relay-B → Relay-C → Gateway

**Like Tor:** Paths change automatically, no single hop knows full route!

---

## Firewall Setup (Windows)

**Allow client to receive responses:**
```powershell
# Run as Administrator
New-NetFirewallRule -DisplayName "Proxy Client" -Direction Inbound -LocalPort 7000 -Protocol TCP -Action Allow
```

---

## Troubleshooting

### "Connection refused"
- ✓ Check server IP addresses in config
- ✓ Ping servers to test connectivity
- ✓ Ensure servers are running

### "No response"
- ✓ Check port 7000 not blocked by firewall
- ✓ Verify downstream servers can reach your PC
- ✓ Check client is listening: `netstat -an | findstr 7000`

### "Timeout"
- ✓ Increase timeout in config (default 30000ms)
- ✓ Check server logs for errors
- ✓ Test with smaller requests first

---

## Files Included

```
client/          - Library for integrating into your code
client-cli/      - Command-line application
client-gui/      - Graphical interface
config/          - Configuration files
common/          - Shared utilities
upstream-server/ - Entry point servers
central-proxy/   - Main proxy server
downstream-server/ - Response delivery (fixed!)
relay-node/      - Optional relay for anonymity
starlink-gateway/ - Optional gateway for shared internet

Documentation:
├─ WINDOWS_GUIDE.md      - Full Windows setup
├─ RELAY_GATEWAY_EXPLAINED.md - Relay/Gateway deep dive
├─ EXAMPLES.md           - Code examples
├─ README.md             - Complete documentation
└─ ARCHITECTURE.md       - System diagrams
```

---

## Next Steps

1. **Deploy servers** (see original README.md)
2. **Configure client.yaml** with your server IPs
3. **Build client** with `build-windows.bat`
4. **Test** with `proxy-cli.exe -url http://example.com`
5. **Integrate** into your applications (see EXAMPLES.md)

For detailed guides, see:
- **WINDOWS_GUIDE.md** - Complete Windows instructions
- **RELAY_GATEWAY_EXPLAINED.md** - Understand relay/gateway
- **EXAMPLES.md** - Integration examples

---

**You now have everything you need!** 🚀
