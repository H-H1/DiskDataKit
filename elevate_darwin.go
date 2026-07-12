//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// isAdmin 检测当前进程是否以 root 身份运行。
func isAdmin() bool {
	return os.Geteuid() == 0
}

// elevate 用 osascript 提权重启（弹出系统授权对话框），成功后退出当前进程。
// 新进程以 root 后台运行，osascript 立即返回，当前进程退出。
func elevate() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法获取可执行文件路径:", err)
		os.Exit(1)
	}
	parts := []string{"exec", shellQuote(exe)}
	for _, a := range os.Args[1:] {
		parts = append(parts, shellQuote(a))
	}
	cmd := strings.Join(parts, " ")
	// 后台运行避免 osascript 阻塞，输出丢弃
	script := fmt.Sprintf(`do shell script "%s > /dev/null 2>&1 &" with administrator privileges`,
		strings.ReplaceAll(cmd, `"`, `\"`))
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, "需要管理员权限运行，提权失败:", err, string(out))
		os.Exit(1)
	}
	os.Exit(0)
}

// shellQuote 用单引号包裹字符串并转义内部单引号。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
