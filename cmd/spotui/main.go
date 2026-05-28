package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yesid/spotui/internal/auth"
	"github.com/yesid/spotui/internal/config"
	"github.com/yesid/spotui/internal/player"
	"github.com/yesid/spotui/internal/spotify"
	"github.com/yesid/spotui/internal/tui"
)

// defaultClientID is baked in at build time so users don't have to paste it.
// A Spotify PKCE client ID is public by design (there is no client secret),
// and only accounts added to the app in development mode can authenticate,
// so embedding it here is safe.
var defaultClientID = "f8b1f64a19a8436bbe19efdc0968e9ab"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Precedence: env var > saved config > baked-in default > interactive prompt.
	if cfg.ClientID == "" {
		cfg.ClientID = defaultClientID
	}
	if id := os.Getenv("SPOTUI_CLIENT_ID"); id != "" {
		cfg.ClientID = id
	}

	if cfg.ClientID == "" {
		cfg.ClientID = promptClientID()
		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
		}
	}

	token, err := auth.LoadToken()
	if err != nil || !token.Valid() {
		token, err = auth.Login(cfg.ClientID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
			os.Exit(1)
		}
		if err := auth.SaveToken(token); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token: %v\n", err)
		}
	}

	tokenMgr := spotify.NewTokenManager(cfg.ClientID, token)
	client := spotify.New(tokenMgr)

	// Start the embedded spotifyd daemon so audio can play on this machine
	// without a separate Spotify client. Optional: if spotifyd isn't installed
	// SpoTUI still works by controlling other Spotify Connect devices.
	daemon := player.New()
	localPlayback := false
	if daemon.Available() {
		if !daemon.HasCredentials() {
			if err := daemon.Authenticate(); err != nil {
				fmt.Fprintf(os.Stderr, "spotifyd auth failed (local playback disabled): %v\n", err)
			}
		}
		if daemon.HasCredentials() {
			if err := daemon.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "spotifyd start failed (local playback disabled): %v\n", err)
			} else {
				localPlayback = true
				defer daemon.Stop()
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "spotifyd not found — install it for local playback. Controlling external devices only.")
	}

	deviceName := ""
	if localPlayback {
		deviceName = player.DeviceName
	}

	prog := tea.NewProgram(tui.NewModel(client, deviceName),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func promptClientID() string {
	fmt.Println("SpoTUI — first time setup")
	fmt.Println()
	fmt.Println("1. Go to https://developer.spotify.com/dashboard")
	fmt.Println("2. Create an app and add http://127.0.0.1:8090/callback as a Redirect URI")
	fmt.Println("3. Copy the Client ID")
	fmt.Println()
	fmt.Print("Paste your Spotify Client ID: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	id := strings.TrimSpace(scanner.Text())
	if id == "" {
		fmt.Fprintln(os.Stderr, "client ID cannot be empty")
		os.Exit(1)
	}
	fmt.Printf("\nSaved to ~/.config/spotui/config.json\n\n")
	return id
}
