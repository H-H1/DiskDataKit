//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// setNoWindow 隐藏子进程控制台窗口，避免 PowerShell 等控制台程序弹出黑窗。
// CREATE_NO_WINDOW (0x08000000) 阻止为子进程创建新的控制台窗口。
func setNoWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
