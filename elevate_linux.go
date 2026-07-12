//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// isAdmin 检测当前进程是否以 root 身份运行。
func isAdmin() bool {
	return os.Geteuid() == 0
}

// elevate 用 pkexec 提权重启（弹出 PolicyKit 授权对话框），当前进程阻塞等待新进程退出。
func elevate() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法获取可执行文件路径:", err)
		os.Exit(1)
	}
	args := append([]string{exe}, os.Args[1:]...)
	cmd := exec.Command("pkexec", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "需要管理员权限运行，提权失败（请确认已安装 pkexec）:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
