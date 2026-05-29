package player

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// oauthPort is the local redirect port librespot's OAuth flow listens on.
const oauthPort = "5588"

// librespotDaemon supervises a librespot subprocess (Windows backend).
//
// librespot has no separate "authenticate" subcommand the way spotifyd does:
// it performs OAuth inline when started with --enable-oauth and caches the
// resulting token. Authenticate() therefore starts a short-lived instance just
// to capture the credentials, then stops it; Start() launches the real daemon
// reusing the cached credentials.
type librespotDaemon struct {
	cachePath string
	logPath   string
	cmd       *exec.Cmd
}

func newLibrespot() *librespotDaemon {
	cfg, _ := os.UserConfigDir()
	base := filepath.Join(cfg, "spotui")
	return &librespotDaemon{
		cachePath: filepath.Join(base, "librespot"),
		logPath:   filepath.Join(base, "librespot.log"),
	}
}

// Available reports whether the librespot binary is installed.
func (d *librespotDaemon) Available() bool {
	_, err := exec.LookPath("librespot")
	return err == nil
}

// HasCredentials reports whether librespot has cached OAuth credentials. It
// stores them as credentials.json at the root of the cache directory.
func (d *librespotDaemon) HasCredentials() bool {
	_, err := os.Stat(filepath.Join(d.cachePath, "credentials.json"))
	return err == nil
}

// Authenticate runs librespot's interactive OAuth flow, opening the browser for
// the user. It blocks until credentials are cached (or it times out), then
// stops the short-lived instance.
func (d *librespotDaemon) Authenticate() error {
	if err := os.MkdirAll(d.cachePath, 0700); err != nil {
		return err
	}

	cmd := exec.Command("librespot",
		"--name", DeviceName,
		"--device-type", "computer",
		"--backend", "rodio",
		"--cache", d.cachePath,
		"--disable-audio-cache",
		"--enable-oauth",
		"--oauth-port", oauthPort,
	)

	// librespot prints the authorize URL to stdout and logs to stderr; merge
	// both into one pipe so we can surface the URL regardless of where it lands.
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("start librespot oauth: %w", err)
	}
	// Close the parent's write end so the reader sees EOF once librespot exits.
	pw.Close()

	fmt.Println("\nAuthorizing librespot for local playback...")
	fmt.Println("A browser window should open — log in and authorize SpoTUI.")
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			if i := strings.Index(line, "https://accounts.spotify.com/authorize"); i >= 0 {
				url := strings.TrimSpace(line[i:])
				fmt.Printf("If your browser didn't open, visit:\n\n  %s\n\n", url)
			}
		}
	}()

	// Wait for credentials to be written, then stop the short-lived instance.
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if d.HasCredentials() {
			fmt.Println("librespot authorized.")
			stopProcess(cmd)
			pr.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	stopProcess(cmd)
	pr.Close()
	return fmt.Errorf("timed out waiting for librespot authorization")
}

// Start launches the librespot daemon in the background. Its output is appended
// to librespot.log. Credentials are reused from the cache, so no OAuth flow is
// triggered here.
func (d *librespotDaemon) Start() error {
	logFile, err := os.OpenFile(d.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("librespot",
		"--name", DeviceName,
		"--device-type", "computer",
		"--backend", "rodio",
		"--bitrate", "320",
		"--cache", d.cachePath,
		"--disable-audio-cache",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start librespot: %w", err)
	}
	d.cmd = cmd
	return nil
}

// Stop terminates the librespot subprocess if running.
func (d *librespotDaemon) Stop() {
	stopProcess(d.cmd)
	d.cmd = nil
}
