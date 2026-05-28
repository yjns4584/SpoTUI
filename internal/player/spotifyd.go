// Package player manages an embedded spotifyd subprocess so SpoTUI can play
// audio directly on this machine without a separate Spotify client. spotifyd
// registers a Spotify Connect device that the REST API can then control.
package player

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// DeviceName is the Spotify Connect device name spotifyd advertises.
const DeviceName = "SpoTUI"

// Daemon supervises a spotifyd subprocess.
type Daemon struct {
	cachePath string
	logPath   string
	cmd       *exec.Cmd
}

func New() *Daemon {
	cfg, _ := os.UserConfigDir()
	base := filepath.Join(cfg, "spotui")
	return &Daemon{
		cachePath: filepath.Join(base, "spotifyd"),
		logPath:   filepath.Join(base, "spotifyd.log"),
	}
}

// Available reports whether the spotifyd binary is installed.
func (d *Daemon) Available() bool {
	_, err := exec.LookPath("spotifyd")
	return err == nil
}

// HasCredentials reports whether spotifyd has cached OAuth credentials from a
// previous Authenticate() call.
func (d *Daemon) HasCredentials() bool {
	_, err := os.Stat(filepath.Join(d.cachePath, "oauth", "credentials.json"))
	return err == nil
}

// Authenticate runs spotifyd's interactive OAuth flow, opening the browser for
// the user. It blocks until login completes and credentials are cached.
// Must be called before Start() when HasCredentials() is false.
func (d *Daemon) Authenticate() error {
	if err := os.MkdirAll(d.cachePath, 0700); err != nil {
		return err
	}

	cmd := exec.Command("spotifyd", "authenticate", "-c", d.cachePath)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start spotifyd authenticate: %w", err)
	}

	fmt.Println("\nAuthorizing spotifyd for local playback...")
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "https://accounts.spotify.com/authorize"); i >= 0 {
			url := strings.TrimSpace(line[i:])
			fmt.Printf("If your browser didn't open, visit:\n\n  %s\n\n", url)
			openBrowser(url)
		}
		if strings.Contains(line, "Login successful") {
			fmt.Println("spotifyd authorized.")
		}
	}
	return cmd.Wait()
}

// Start launches the spotifyd daemon in the background. Its output is appended
// to spotifyd.log. Audio plays through the rodio (cpal/ALSA) backend.
func (d *Daemon) Start() error {
	logFile, err := os.OpenFile(d.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("spotifyd",
		"--no-daemon",
		"-c", d.cachePath,
		"--no-audio-cache",
		"--device-name", DeviceName,
		"--device-type", "computer",
		"--backend", "rodio",
		"--bitrate", "320",
		"--volume-controller", "soft-volume",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start spotifyd: %w", err)
	}
	d.cmd = cmd
	return nil
}

// Stop terminates the spotifyd subprocess if running.
func (d *Daemon) Stop() {
	if d.cmd == nil || d.cmd.Process == nil {
		return
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = d.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = d.cmd.Process.Kill()
	}
	d.cmd = nil
}

func openBrowser(url string) {
	for _, cmd := range []string{"xdg-open", "open", "start"} {
		if err := exec.Command(cmd, url).Start(); err == nil {
			return
		}
	}
}
