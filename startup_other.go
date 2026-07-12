//go:build !windows

package main

// listStartupItems 在非 Windows 平台返回空（Logon/Explorer/IE 为 Windows 专有概念）。
func listStartupItems() []StartupItem { return nil }

// toggleStartupItem 在非 Windows 平台空操作。
func toggleStartupItem(category, name, location string, enable bool) error {
	return nil
}
