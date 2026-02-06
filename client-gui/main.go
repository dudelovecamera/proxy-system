package main

import (
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/yourusername/proxy-system/client"
)

type ProxyGUI struct {
	app          fyne.App
	window       fyne.Window
	client       *client.ProxyClient
	urlEntry     *widget.Entry
	methodSelect *widget.Select
	bodyEntry    *widget.Entry
	responseText *widget.Entry
	statusLabel  *widget.Label
	sendButton   *widget.Button
}

func main() {
	// Initialize proxy client
	proxyClient, err := client.NewProxyClient("config/client.yaml")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Start response listener
	go func() {
		if err := proxyClient.Start(); err != nil {
			log.Fatalf("Client server error: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	// Create GUI
	gui := &ProxyGUI{
		app:    app.New(),
		client: proxyClient,
	}

	gui.setupUI()
	gui.window.ShowAndRun()
}

func (g *ProxyGUI) setupUI() {
	g.window = g.app.NewWindow("Distributed Proxy Client")
	g.window.Resize(fyne.NewSize(800, 600))

	// URL input
	g.urlEntry = widget.NewEntry()
	g.urlEntry.SetPlaceHolder("Enter URL (e.g., http://example.com)")

	// Method selector
	g.methodSelect = widget.NewSelect([]string{"GET", "POST", "PUT", "DELETE"}, nil)
	g.methodSelect.SetSelected("GET")

	// Body input (for POST/PUT)
	g.bodyEntry = widget.NewMultiLineEntry()
	g.bodyEntry.SetPlaceHolder("Request body (JSON, form data, etc.)")
	g.bodyEntry.SetMinRowsVisible(5)

	// Response display
	g.responseText = widget.NewMultiLineEntry()
	g.responseText.SetPlaceHolder("Response will appear here...")
	g.responseText.Disable()
	g.responseText.SetMinRowsVisible(15)

	// Status label
	g.statusLabel = widget.NewLabel("Ready")

	// Send button
	g.sendButton = widget.NewButton("Send Request", g.handleSendRequest)

	// Layout
	requestForm := container.NewVBox(
		widget.NewLabel("Request URL:"),
		g.urlEntry,
		widget.NewLabel("Method:"),
		g.methodSelect,
		widget.NewLabel("Body:"),
		g.bodyEntry,
		g.sendButton,
	)

	responseSection := container.NewVBox(
		widget.NewLabel("Response:"),
		g.responseText,
		g.statusLabel,
	)

	content := container.NewVSplit(
		requestForm,
		responseSection,
	)
	content.SetOffset(0.4)

	// Menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Clear", func() {
			g.responseText.SetText("")
			g.statusLabel.SetText("Ready")
		}),
		fyne.NewMenuItem("Quit", func() {
			g.app.Quit()
		}),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			dialog := widget.NewLabel("Distributed Multi-Path Proxy Client\nVersion 1.0")
			popup := widget.NewModalPopUp(dialog, g.window.Canvas())
			popup.Show()
		}),
	)

	mainMenu := fyne.NewMainMenu(fileMenu, helpMenu)
	g.window.SetMainMenu(mainMenu)

	g.window.SetContent(content)
}

func (g *ProxyGUI) handleSendRequest() {
	url := g.urlEntry.Text
	method := g.methodSelect.Selected
	body := []byte(g.bodyEntry.Text)

	if url == "" {
		g.statusLabel.SetText("Error: URL is required")
		return
	}

	g.statusLabel.SetText("Sending request...")
	g.sendButton.Disable()
	g.responseText.SetText("Loading...")

	// Make request in background
	go func() {
		headers := map[string]string{
			"User-Agent":   "Distributed-Proxy-GUI/1.0",
			"Content-Type": "application/json",
		}

		startTime := time.Now()
		response, err := g.client.MakeRequest(method, url, body, headers)
		duration := time.Since(startTime)

		// Update UI on main thread
		g.window.Canvas().Refresh(g.statusLabel)

		if err != nil {
			g.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
			g.responseText.SetText(fmt.Sprintf("Request failed: %v", err))
		} else {
			g.statusLabel.SetText(fmt.Sprintf("✓ Response received in %v", duration))
			responseBody := string(response.Body)
			if len(responseBody) > 10000 {
				responseBody = responseBody[:10000] + "\n\n... (truncated, too large)"
			}
			g.responseText.SetText(responseBody)
		}

		g.sendButton.Enable()
		g.window.Canvas().Refresh(g.responseText)
		g.window.Canvas().Refresh(g.sendButton)
	}()
}
