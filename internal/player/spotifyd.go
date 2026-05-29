package player

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// spotifydDaemon supervises a spotifyd subprocess (Linux/macOS backend).
type spotifydDaemon struct {
	cachePath string
	logPath   string
	cmd       *exec.Cmd
}

func newSpotifyd() *spotifydDaemon {
	cfg, _ := os.UserConfigDir()
	base := filepath.Join(cfg, "spotui")
	return &spotifydDaemon{
		cachePath: filepath.Join(base, "spotifyd"),
		logPath:   filepath.Join(base, "spotifyd.log"),
	}
}

// Available reports whether the spotifyd binary is installed.
func (d *spotifydDaemon) Available() bool {
	_, err := exec.LookPath("spotifyd")
	return err == nil
}

// HasCredentials reports whether spotifyd has cached OAuth credentials from a
// previous Authenticate() call.
func (d *spotifydDaemon) HasCredentials() bool {
	_, err := os.Stat(filepath.Join(d.cachePath, "oauth", "credentials.json"))
	return err == nil
}

// Authenticate runs spotifyd's interactive OAuth flow, opening the browser for
// the user. It blocks until login completes and credentials are cached.
func (d *spotifydDaemon) Authenticate() error {
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
func (d *spotifydDaemon) Start() error {
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
func (d *spotifydDaemon) Stop() {
	stopProcess(d.cmd)
	d.cmd = nil
}
