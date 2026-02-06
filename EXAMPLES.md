# Example Usage Code

## Example 1: Simple GET Request

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
        log.Fatalf("Failed to create client: %v", err)
    }

    // Start listening for responses
    go proxyClient.Start()

    // Make GET request
    response, err := proxyClient.GET("http://example.com", nil)
    if err != nil {
        log.Fatalf("Request failed: %v", err)
    }

    log.Printf("Response: %s", string(response.Body))
}
```

## Example 2: POST Request with JSON

```go
package main

import (
    "encoding/json"
    "log"
    "github.com/yourusername/proxy-system/client"
)

func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    // Prepare JSON data
    data := map[string]interface{}{
        "username": "testuser",
        "password": "secret123",
    }
    
    jsonData, _ := json.Marshal(data)

    // Set headers
    headers := map[string]string{
        "Content-Type": "application/json",
        "Authorization": "Bearer token123",
    }

    // Make POST request
    response, err := proxyClient.POST(
        "http://api.example.com/login",
        jsonData,
        headers,
    )

    if err != nil {
        log.Fatalf("Login failed: %v", err)
    }

    log.Printf("Login response: %s", string(response.Body))
}
```

## Example 3: Web Scraper

```go
package main

import (
    "log"
    "regexp"
    "github.com/yourusername/proxy-system/client"
)

func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    // List of URLs to scrape
    urls := []string{
        "http://example.com/page1",
        "http://example.com/page2",
        "http://example.com/page3",
    }

    // Scrape each URL
    for _, url := range urls {
        response, err := proxyClient.GET(url, nil)
        if err != nil {
            log.Printf("Failed to fetch %s: %v", url, err)
            continue
        }

        // Extract email addresses
        emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
        emails := emailRegex.FindAllString(string(response.Body), -1)

        log.Printf("Found %d emails on %s", len(emails), url)
        for _, email := range emails {
            log.Printf("  - %s", email)
        }
    }
}
```

## Example 4: API Client

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "github.com/yourusername/proxy-system/client"
)

type APIClient struct {
    proxy *client.ProxyClient
    baseURL string
    token string
}

func NewAPIClient(baseURL, token string) *APIClient {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    return &APIClient{
        proxy: proxyClient,
        baseURL: baseURL,
        token: token,
    }
}

func (c *APIClient) Get(endpoint string) (map[string]interface{}, error) {
    url := c.baseURL + endpoint
    
    headers := map[string]string{
        "Authorization": "Bearer " + c.token,
        "Accept": "application/json",
    }

    response, err := c.proxy.GET(url, headers)
    if err != nil {
        return nil, err
    }

    var result map[string]interface{}
    json.Unmarshal(response.Body, &result)
    
    return result, nil
}

func (c *APIClient) Post(endpoint string, data interface{}) (map[string]interface{}, error) {
    url := c.baseURL + endpoint
    
    jsonData, _ := json.Marshal(data)
    
    headers := map[string]string{
        "Authorization": "Bearer " + c.token,
        "Content-Type": "application/json",
    }

    response, err := c.proxy.POST(url, jsonData, headers)
    if err != nil {
        return nil, err
    }

    var result map[string]interface{}
    json.Unmarshal(response.Body, &result)
    
    return result, nil
}

func main() {
    // Create API client
    api := NewAPIClient("https://api.example.com", "your-token-here")

    // Get user data
    user, err := api.Get("/users/123")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("User: %v\n", user)

    // Create new post
    postData := map[string]string{
        "title": "Test Post",
        "body": "This is a test post via proxy",
    }
    
    result, err := api.Post("/posts", postData)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created post: %v\n", result)
}
```

## Example 5: Download File

```go
package main

import (
    "io/ioutil"
    "log"
    "github.com/yourusername/proxy-system/client"
)

func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    // Download file
    response, err := proxyClient.GET(
        "http://example.com/image.jpg",
        map[string]string{
            "User-Agent": "FileDownloader/1.0",
        },
    )

    if err != nil {
        log.Fatalf("Download failed: %v", err)
    }

    // Save to file
    err = ioutil.WriteFile("downloaded_image.jpg", response.Body, 0644)
    if err != nil {
        log.Fatalf("Failed to save file: %v", err)
    }

    log.Printf("Downloaded %d bytes", len(response.Body))
}
```

## Example 6: Concurrent Requests

```go
package main

import (
    "log"
    "sync"
    "github.com/yourusername/proxy-system/client"
)

func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    urls := []string{
        "http://example.com/page1",
        "http://example.com/page2",
        "http://example.com/page3",
        "http://example.com/page4",
        "http://example.com/page5",
    }

    var wg sync.WaitGroup

    for _, url := range urls {
        wg.Add(1)
        go func(u string) {
            defer wg.Done()

            response, err := proxyClient.GET(u, nil)
            if err != nil {
                log.Printf("Failed %s: %v", u, err)
                return
            }

            log.Printf("✓ %s: %d bytes", u, len(response.Body))
        }(url)
    }

    wg.Wait()
    log.Println("All requests completed")
}
```

## Example 7: Browser Automation Proxy

```go
package main

import (
    "context"
    "log"
    "net/http"
    "net/http/httputil"
    "net/url"
    "github.com/yourusername/proxy-system/client"
)

// Create local HTTP proxy that browsers can use
func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    // Create HTTP server that acts as proxy
    handler := &ProxyHandler{client: proxyClient}
    
    server := &http.Server{
        Addr:    ":8888",
        Handler: handler,
    }

    log.Println("Browser proxy listening on :8888")
    log.Println("Set your browser proxy to: localhost:8888")
    
    log.Fatal(server.ListenAndServe())
}

type ProxyHandler struct {
    client *client.ProxyClient
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Get full URL
    targetURL := r.URL.String()
    if r.URL.Host == "" {
        targetURL = r.Host + r.URL.Path
        if r.URL.RawQuery != "" {
            targetURL += "?" + r.URL.RawQuery
        }
    }

    // Read request body
    body, _ := httputil.DumpRequest(r, true)

    // Convert headers
    headers := make(map[string]string)
    for k, v := range r.Header {
        if len(v) > 0 {
            headers[k] = v[0]
        }
    }

    // Make proxied request
    response, err := h.client.MakeRequest(r.Method, targetURL, body, headers)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }

    // Write response
    w.WriteHeader(response.StatusCode)
    w.Write(response.Body)
}
```

## Example 8: Retry Logic

```go
package main

import (
    "log"
    "time"
    "github.com/yourusername/proxy-system/client"
)

func makeRequestWithRetry(proxyClient *client.ProxyClient, url string, maxRetries int) ([]byte, error) {
    var lastErr error

    for i := 0; i < maxRetries; i++ {
        response, err := proxyClient.GET(url, nil)
        if err == nil {
            return response.Body, nil
        }

        lastErr = err
        log.Printf("Attempt %d failed: %v. Retrying in %d seconds...", i+1, err, i+1)
        time.Sleep(time.Duration(i+1) * time.Second)
    }

    return nil, lastErr
}

func main() {
    proxyClient, _ := client.NewProxyClient("config/client.yaml")
    go proxyClient.Start()

    body, err := makeRequestWithRetry(proxyClient, "http://example.com", 3)
    if err != nil {
        log.Fatalf("All retries failed: %v", err)
    }

    log.Printf("Success: %d bytes", len(body))
}
```

## Command-Line Examples

### Basic Usage

```bash
# Simple GET request
proxy-cli.exe -url http://example.com

# POST request
proxy-cli.exe -method POST -url http://api.example.com/data -data '{"key":"value"}'

# With custom header
proxy-cli.exe -url http://example.com -H "Authorization: Bearer token123"

# Verbose output
proxy-cli.exe -url http://example.com -v

# Interactive mode
proxy-cli.exe -i
```

### Advanced Usage

```bash
# POST with file
proxy-cli.exe -method POST -url http://api.example.com/upload -data-file request.json

# Multiple headers (need to modify code to support multiple -H flags)
proxy-cli.exe -url http://api.example.com -H "Authorization: Bearer token" -H "Content-Type: application/json"

# Save response to file (in PowerShell)
proxy-cli.exe -url http://example.com > response.html

# Download file
proxy-cli.exe -url http://example.com/file.zip > downloaded.zip
```

### Batch Script Example

```batch
@echo off
REM Fetch multiple pages

proxy-cli.exe -url http://example.com/page1 > page1.html
proxy-cli.exe -url http://example.com/page2 > page2.html
proxy-cli.exe -url http://example.com/page3 > page3.html

echo All pages downloaded!
```

### PowerShell Script Example

```powershell
# Fetch and process multiple URLs
$urls = @(
    "http://example.com/page1",
    "http://example.com/page2",
    "http://example.com/page3"
)

foreach ($url in $urls) {
    Write-Host "Fetching: $url"
    .\proxy-cli.exe -url $url | Out-File "output_$(Get-Random).html"
}

Write-Host "Complete!"
```
