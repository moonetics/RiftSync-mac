//go:build darwin

package uiapp

import (
	"errors"
	"os/exec"
	"strings"
)

func openFolder(target string) error {
	return exec.Command("open", target).Start()
}

func openInIDE(target, ide string) error {
	ide = strings.ToLower(strings.TrimSpace(ide))
	switch ide {
	case "antigravity":
		if err := exec.Command("open", "-b", "com.google.antigravity-ide", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-b", "com.google.antigravity", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-a", "Antigravity IDE", target).Run(); err == nil {
			return nil
		}
		return exec.Command("open", "-a", "Antigravity", target).Run()

	case "vscode":
		if err := exec.Command("open", "-b", "com.microsoft.VSCode", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-a", "Visual Studio Code", target).Run(); err == nil {
			return nil
		}
		return exec.Command("code", target).Run()

	case "cursor":
		if err := exec.Command("open", "-b", "com.todesktop.230313mzl4w4u92", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-a", "Cursor", target).Run(); err == nil {
			return nil
		}
		return exec.Command("cursor", target).Run()

	case "default":
		return exec.Command("open", target).Start()

	case "auto", "":
		// Priority 1: Antigravity IDE
		if err := exec.Command("open", "-b", "com.google.antigravity-ide", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-b", "com.google.antigravity", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-a", "Antigravity IDE", target).Run(); err == nil {
			return nil
		}
		// Priority 2: Visual Studio Code
		if err := exec.Command("open", "-b", "com.microsoft.VSCode", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("open", "-a", "Visual Studio Code", target).Run(); err == nil {
			return nil
		}
		if err := exec.Command("code", target).Run(); err == nil {
			return nil
		}
		// Priority 3: Cursor
		if err := exec.Command("open", "-b", "com.todesktop.230313mzl4w4u92", target).Run(); err == nil {
			return nil
		}
		// Fallback: System default
		return exec.Command("open", target).Start()

	default:
		if err := exec.Command("open", "-a", ide, target).Run(); err == nil {
			return nil
		}
		return exec.Command("open", target).Start()
	}
}

// macOS keeps its native title bar. The embedded page remains fully usable,
// while close is finalized through webview.Terminate in requestQuit.
func (w *windowController) makeFrameless() error { return nil }
func (w *windowController) drag() error          { return nil }
func (w *windowController) close() error         { return nil }

func (w *windowController) minimize() error {
	if w == nil || w.native == 0 {
		return errors.New("window handle is unavailable")
	}
	return exec.Command("osascript", "-e", `tell application "System Events" to tell first process whose frontmost is true to set value of attribute "AXMinimized" of window 1 to true`).Run()
}

func pickFolder(_ uintptr) (string, bool, error) {
	output, err := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Choose RiftSync folder")`).Output()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "exit status 1") {
			return "", false, nil
		}
		return "", false, err
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", false, nil
	}
	return strings.TrimSuffix(path, "/"), true, nil
}
