//go:build !windows

package gitversion

import "os/exec"

func hideCommandWindow(cmd *exec.Cmd) {}
