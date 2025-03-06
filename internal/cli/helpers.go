package cli

import (
	"os/exec"
	"runtime"
)

func openUrl(uri string) {
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("xdg-open", uri).Start()
	case "darwin":
		_ = exec.Command("open", uri).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", uri).Start()
	}
}
