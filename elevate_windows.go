//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// isAdmin 检测当前进程是否以管理员身份运行（UAC 提权后）。
func isAdmin() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

var procShellExecuteW = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

// elevate 以管理员身份重新启动当前程序（弹出 UAC 对话框），成功后退出当前进程。
// 工作目录保持与当前进程一致，避免相对路径错位。
func elevate() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "无法获取可执行文件路径:", err)
		os.Exit(1)
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString(strings.Join(os.Args[1:], " "))
	wd, _ := os.Getwd()
	dir, _ := syscall.UTF16PtrFromString(wd)
	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		uintptr(unsafe.Pointer(dir)),
		uintptr(1), // SW_SHOWNORMAL
	)
	// ShellExecuteW 返回值 <= 32 表示失败（如用户拒绝 UAC）
	if ret <= 32 {
		fmt.Fprintln(os.Stderr, "需要管理员权限运行，UAC 提权失败:", callErr)
		os.Exit(1)
	}
	os.Exit(0)
}
