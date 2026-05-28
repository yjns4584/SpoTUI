package auth

import "os/exec"

func openBrowser(url string) {
	// try common Linux launchers, then macOS, then Windows
	for _, cmd := range []string{"xdg-open", "open", "start"} {
		if err := exec.Command(cmd, url).Start(); err == nil {
			return
		}
	}
}
