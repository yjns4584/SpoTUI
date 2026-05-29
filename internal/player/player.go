// Package player manages an embedded local-playback daemon so SpoTUI can play
// audio directly on this machine without a separate Spotify client. The daemon
// registers a Spotify Connect device that the REST API can then control.
//
// Two backends are supported, picked automatically per operating system:
//
//   - spotifyd  — used on Linux and macOS.
//   - librespot — used on Windows, where spotifyd has no support. librespot is
//     the same engine spotifyd is built on and runs natively on Windows.
package player

import (
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// DeviceName is the Spotify Connect device name the daemon advertises.
const DeviceName = "SpoTUI"

// Backend supervises a local-playback daemon (spotifyd or librespot).
type Backend interface {
	// Available reports whether the daemon binary is installed.
	Available() bool
	// HasCredentials reports whether the daemon has cached OAuth credentials
	// from a previous Authenticate() call.
	HasCredentials() bool
	// Authenticate runs the daemon's interactive OAuth flow, opening the
	// browser for the user. It blocks until login completes and credentials
	// are cached. Must be called before Start() when HasCredentials() is false.
	Authenticate() error
	// Start launches the daemon in the background.
	Start() error
	// Stop terminates the daemon subprocess if running.
	Stop()
}

// New returns the local-playback backend appropriate for the current OS:
// librespot on Windows, spotifyd everywhere else.
func New() Backend {
	if runtime.GOOS == "windows" {
		return newLibrespot()
	}
	return newSpotifyd()
}

// openBrowser opens url in the user's default browser, trying the right
// launcher for the current platform.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// stopProcess gracefully terminates a running command, falling back to a hard
// kill after a short grace period. On Windows it kills outright, since POSIX
// signals like SIGTERM aren't delivered to console subprocesses there.
func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
	} else {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
	}
}
