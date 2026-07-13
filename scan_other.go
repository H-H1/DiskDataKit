//go:build !windows

package main

// scanAll 在非 Windows 平台返回空。
func scanAll() []ScanItem { return nil }

// scanInstalled 在非 Windows 平台返回空。
func scanInstalled() []ScanItem { return nil }
