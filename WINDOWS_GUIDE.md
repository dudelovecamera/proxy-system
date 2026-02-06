# Windows Installation & Usage Guide

## Complete Step-by-Step Setup for Windows

### Prerequisites

1. **Install Go (Golang)**
   - Download from: https://go.dev/dl/
   - Get: `go1.21.windows-amd64.msi` (or latest version)
   - Run installer
   - Verify installation:
     ```cmd
     go version
     ```

2. **Install Git (Optional, for cloning)**
   - Download from: https://git-scm.com/download/win
   - Run installer with default settings

3. **Install GCC (for GUI client)**
   - Download MSYS2: https://www.msys2.org/
   - Install MSYS2
   - Open MSYS2 terminal and run:
     ```bash
     pacman -S mingw-w64-x86_64-gcc
     ```
   - Add to PATH: `C:\msys64\mingw64\bin`

### Installation Steps

#### Step 1: Extract the Project

```cmd
# Extract the tar.gz file
# You can use 7-Zip (https://www.7-zip.org/)
# Right-click → 7-Zip → Extract Here

cd distributed-proxy-system
```

#### Step 2: Install Dependencies

```cmd
# Initialize Go module
go mod init github.com/yourusername/proxy-system

# Install dependencies
go get gopkg.in/yaml.v3

# For GUI client (optional)
go get fyne.io/fyne/v2
```

#### Step 3: Build the Client Applications

**Build CLI Client:**
```cmd
cd client-cli
go build -o proxy-cli.exe main.go
```

**Build GUI Client (Optional):**
```cmd
cd client-gui
go build -o proxy-gui.exe main.go
```

**Build Library Client:**
```cmd
cd client
go build -o client.dll -buildmode=c-shared main.go
```

### Configuration

#### Edit `config/client.yaml`

```yaml
chunk_size: 8192

# Replace with your actual server IPs/domains
upstream_servers:
  - "upstream1.example.com:8001"
  - "upstream2.example.com:8002"
  - "upstream3.example.com:8003"

downstream_port: 7000
timeout: 30000

encryption:
  enabled: true
  algorithm: "aes-256-gcm"
  mode: "body_only"
```

**Important:** Make sure the `upstream_servers` point to your actual server addresses!

### Running the Client

#### Method 1: Command-Line Interface (CLI)

**Basic GET Request:**
```cmd
proxy-cli.exe -url http://example.com
```

**POST Request:**
```cmd
proxy-cli.exe -method POST -url http://api.example.com/data -data "{\"key\":\"value\"}"
```

**With Custom Headers:**
```cmd
proxy-cli.exe -url http://example.com -H "Authorization: Bearer token123"
```

**Interactive Mode:**
```cmd
proxy-cli.exe -i
```

**Verbose Output:**
```cmd
proxy-cli.exe -url http://example.com -v
```

#### Method 2: GUI Application

**Just double-click:**
```cmd
proxy-gui.exe
```

Or run from command line:
```cmd
.\proxy-gui.exe
```

**GUI Usage:**
1. Enter URL in the URL field
2. Select HTTP method (GET, POST, etc.)
3. Enter request body if needed (for POST/PUT)
4. Click "Send Request"
5. View response in the bottom panel

#### Method 3: Use as Library in Your Code

```go
package main

import (
    "log"
    "github.com/yourusername/proxy-system/client"
)

func main() {
    // Create client
    proxyClient, err := client.NewProxyClient("config/client.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Start listening for responses
    go proxyClient.Start()

    // Make GET request
    headers := map[string]string{
        "User-Agent": "MyApp/1.0",
    }
    
    response, err := proxyClient.GET("http://example.com", headers)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Response: %s", string(response.Body))

    // Make POST request
    body := []byte(`{"username":"test","password":"123"}`)
    response, err = proxyClient.POST("http://api.example.com/login", body, headers)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Login response: %s", string(response.Body))
}
```

### Windows Firewall Configuration

**Allow incoming connections on port 7000 (for receiving responses):**

```cmd
# Run as Administrator in PowerShell
New-NetFirewallRule -DisplayName "Proxy Client Response Port" -Direction Inbound -LocalPort 7000 -Protocol TCP -Action Allow
```

Or use GUI:
1. Open Windows Defender Firewall
2. Advanced Settings
3. Inbound Rules → New Rule
4. Port → TCP → 7000
5. Allow the connection
6. Apply to all profiles
7. Name: "Proxy Client"

### Running as Windows Service (Optional)

#### Create Windows Service with NSSM

1. **Download NSSM:**
   - https://nssm.cc/download
   - Extract `nssm.exe`

2. **Install Service:**
   ```cmd
   nssm install ProxyClient "C:\path\to\proxy-cli.exe"
   nssm set ProxyClient AppDirectory "C:\path\to\distributed-proxy-system"
   nssm set ProxyClient AppParameters "-i"
   nssm start ProxyClient
   ```

3. **Manage Service:**
   ```cmd
   # Start
   nssm start ProxyClient

   # Stop
   nssm stop ProxyClient

   # Remove
   nssm remove ProxyClient confirm
   ```

### Using with Web Browsers

#### Configure as System Proxy (Advanced)

Create a local HTTP proxy server wrapper:

```go
// Save as http-proxy.go
package main

import (
    "io/ioutil"
    "log"
    "net/http"
    "github.com/yourusername/proxy-system/client"
)

func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Read request body
        body, _ := ioutil.ReadAll(r.Body)
        
        // Make proxied request
        response, err := proxyClient.MakeRequest(
            r.Method,
            r.URL.String(),
            body,
            convertHeaders(r.Header),
        )
        
        if err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        
        w.Write(response.Body)
    })

    log.Println("HTTP Proxy listening on :8888")
    http.ListenAndServe(":8888", nil)
}

func convertHeaders(h http.Header) map[string]string {
    headers := make(map[string]string)
    for k, v := range h {
        if len(v) > 0 {
            headers[k] = v[0]
        }
    }
    return headers
}
```

**Build and run:**
```cmd
go build -o http-proxy.exe http-proxy.go
http-proxy.exe
```

**Configure Windows proxy:**
1. Settings → Network & Internet → Proxy
2. Manual proxy setup
3. Address: `localhost`
4. Port: `8888`
5. Save

### Troubleshooting

#### Issue 1: "go: command not found"
**Solution:** Restart Command Prompt/PowerShell after installing Go

#### Issue 2: Cannot build GUI client
**Solution:** Make sure GCC is installed and in PATH
```cmd
gcc --version
```

#### Issue 3: Connection refused to upstream servers
**Solution:** Check:
- Upstream servers are running
- IP addresses in config are correct
- Firewall allows outbound connections
- Use `ping` and `telnet` to test connectivity

#### Issue 4: No response received
**Solution:** Check:
- Port 7000 is not blocked by firewall
- Downstream servers can reach your client IP
- Check logs for timeout messages

#### Issue 5: SSL/TLS errors
**Solution:** 
```cmd
# Trust certificates (if using custom CA)
certutil -addstore -f "ROOT" your-ca-cert.crt
```

### Performance Optimization

#### For better performance on Windows:

**Increase file handles:**
```cmd
# Run as Administrator
REG ADD "HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" /v MaxUserPort /t REG_DWORD /d 65534 /f
REG ADD "HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" /v TcpTimedWaitDelay /t REG_DWORD /d 30 /f
```

**Disable Nagle's algorithm (for lower latency):**
Edit client code, add:
```go
import "net"

// In HTTP client configuration
dialer := &net.Dialer{
    Timeout: 30 * time.Second,
}

// Set TCP_NODELAY
conn, _ := dialer.Dial("tcp", address)
tcpConn := conn.(*net.TCPConn)
tcpConn.SetNoDelay(true)
```

### Development on Windows

**Hot reload during development:**
```cmd
# Install air for auto-reload
go install github.com/cosmtrek/air@latest

# Run with auto-reload
air
```

**Debugging:**
```cmd
# Install delve debugger
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug
dlv debug main.go
```

### Build for Distribution

**Create standalone executable (includes all dependencies):**
```cmd
go build -ldflags="-s -w" -o proxy-client.exe main.go
```

**Create installer with Inno Setup:**
1. Download Inno Setup: https://jrsoftware.org/isdl.php
2. Create installer script (see `installer.iss` below)
3. Compile with Inno Setup

### Complete Build Script for Windows

**Save as `build-windows.bat`:**
```batch
@echo off
echo Building Proxy Client for Windows...

echo.
echo [1/4] Building CLI client...
cd client-cli
go build -ldflags="-s -w" -o ..\build\proxy-cli.exe main.go
cd ..

echo [2/4] Building GUI client...
cd client-gui
go build -ldflags="-s -w -H=windowsgui" -o ..\build\proxy-gui.exe main.go
cd ..

echo [3/4] Copying configuration...
copy config\client.yaml build\
copy README.md build\

echo [4/4] Creating archive...
powershell Compress-Archive -Path build\* -DestinationPath proxy-client-windows.zip -Force

echo.
echo ✓ Build complete!
echo Output: proxy-client-windows.zip
pause
```

**Run the build script:**
```cmd
build-windows.bat
```

### Testing Your Setup

**Test connectivity:**
```cmd
# Test if upstream servers are reachable
telnet upstream1.example.com 8001

# Test local client
curl http://localhost:7000/health
```

**Run test request:**
```cmd
proxy-cli.exe -url http://httpbin.org/get -v
```

**Expected output:**
```
2024/02/06 10:30:15 Making request to http://httpbin.org/get (Session: abc123...)
2024/02/06 10:30:15 Fragmenting request into 1 chunks of ~8192 bytes
2024/02/06 10:30:15 Sent chunk 1/1 to localhost:8001
2024/02/06 10:30:16 Received response chunk 1/1 for session abc123
2024/02/06 10:30:16 Assembling response for session abc123
2024/02/06 10:30:16 Response assembled: 324 bytes

{
  "args": {},
  "headers": {
    "Host": "httpbin.org",
    ...
  }
}
```

### Next Steps

1. ✅ Configure your upstream/downstream servers
2. ✅ Update `config/client.yaml` with server addresses
3. ✅ Build the client
4. ✅ Test with a simple GET request
5. ✅ Configure Windows firewall if needed
6. ✅ Integrate into your applications

### Support

For issues:
1. Check logs in the client output
2. Verify server connectivity
3. Check configuration file syntax
4. Review firewall settings
5. Test with verbose mode: `-v`
