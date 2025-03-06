package cli

import (
	"os/exec"
	"runtime"
)

func openUrl(uri string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", uri).Start()
	}
}
