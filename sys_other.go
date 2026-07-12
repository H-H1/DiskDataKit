//go:build !windows

package main

import "os/exec"

// setNoWindow 非 Windows 平台空操作。
func setNoWindow(cmd *exec.Cmd) {}
