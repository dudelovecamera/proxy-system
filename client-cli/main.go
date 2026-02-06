package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dudelovecamera/proxy-system/client"
)

func main() {
	// Command-line flags
	configPath := flag.String("config", "config/client.yaml", "Path to config file")
	method := flag.String("method", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	url := flag.String("url", "", "Target URL")
	data := flag.String("data", "", "Request body data (for POST/PUT)")
	dataFile := flag.String("data-file", "", "File containing request body")
	header := flag.String("H", "", "Header in format 'Key: Value' (can be used multiple times)")
	verbose := flag.Bool("v", false, "Verbose output")
	interactive := flag.Bool("i", false, "Interactive mode")

	flag.Parse()

	// Parse headers
	headers := make(map[string]string)
	if *header != "" {
		parts := strings.SplitN(*header, ":", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Initialize client
	proxyClient, err := client.NewProxyClient(*configPath)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Start response listener
	go func() {
		if err := proxyClient.Start(); err != nil {
			log.Fatalf("Client server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	if *verbose {
		log.Println("Proxy client initialized")
	}

	// Interactive mode
	if *interactive {
		runInteractive(proxyClient, *verbose)
		return
	}

	// Command-line mode
	if *url == "" {
		fmt.Println("Usage: proxy-cli -url <URL> [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Prepare request body
	var body []byte
	if *dataFile != "" {
		body, err = ioutil.ReadFile(*dataFile)
		if err != nil {
			log.Fatalf("Failed to read data file: %v", err)
		}
	} else if *data != "" {
		body = []byte(*data)
	}

	// Make request
	if *verbose {
		log.Printf("Making %s request to %s", *method, *url)
		if len(body) > 0 {
			log.Printf("Request body: %d bytes", len(body))
		}
	}

	startTime := time.Now()
	response, err := proxyClient.MakeRequest(*method, *url, body, headers)
	duration := time.Since(startTime)

	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	// Display response
	if *verbose {
		log.Printf("Response received in %v", duration)
		log.Printf("Status: %d", response.StatusCode)
		log.Printf("Body size: %d bytes", len(response.Body))
		log.Println("\nResponse headers:")
		for k, v := range response.Headers {
			log.Printf("  %s: %s", k, v)
		}
		log.Println("\nResponse body:")
	}

	fmt.Println(string(response.Body))
}

func runInteractive(proxyClient *client.ProxyClient, verbose bool) {
	fmt.Println("=================================")
	fmt.Println("  Distributed Proxy CLI")
	fmt.Println("=================================")
	fmt.Println()

	for {
		fmt.Println("\nCommands:")
		fmt.Println("  1. GET request")
		fmt.Println("  2. POST request")
		fmt.Println("  3. Status")
		fmt.Println("  4. Exit")
		fmt.Print("\nChoose option: ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			handleGET(proxyClient, verbose)
		case 2:
			handlePOST(proxyClient, verbose)
		case 3:
			showStatus(proxyClient)
		case 4:
			fmt.Println("Goodbye!")
			os.Exit(0)
		default:
			fmt.Println("Invalid option")
		}
	}
}

func handleGET(proxyClient *client.ProxyClient, verbose bool) {
	var url string
	fmt.Print("Enter URL: ")
	fmt.Scanln(&url)

	headers := make(map[string]string)
	headers["User-Agent"] = "Distributed-Proxy-CLI/1.0"

	if verbose {
		log.Printf("Making GET request to %s", url)
	}

	startTime := time.Now()
	response, err := proxyClient.GET(url, headers)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("\n✓ Response received in %v\n", duration)
	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Size: %d bytes\n\n", len(response.Body))

	// Show first 500 characters
	preview := string(response.Body)
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	fmt.Println(preview)
}

func handlePOST(proxyClient *client.ProxyClient, verbose bool) {
	var url, data string
	fmt.Print("Enter URL: ")
	fmt.Scanln(&url)
	fmt.Print("Enter data: ")
	fmt.Scanln(&data)

	headers := map[string]string{
		"User-Agent":   "Distributed-Proxy-CLI/1.0",
		"Content-Type": "application/json",
	}

	if verbose {
		log.Printf("Making POST request to %s", url)
	}

	startTime := time.Now()
	response, err := proxyClient.POST(url, []byte(data), headers)
	duration := time.Since(startTime)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("\n✓ Response received in %v\n", duration)
	fmt.Printf("Status: %d\n", response.StatusCode)
	fmt.Printf("Size: %d bytes\n\n", len(response.Body))
	fmt.Println(string(response.Body))
}

func showStatus(proxyClient *client.ProxyClient) {
	fmt.Println("\n=== Client Status ===")
	fmt.Println("Status: Running")
	fmt.Println("Listening for responses")
	// Add more status info as needed
}
