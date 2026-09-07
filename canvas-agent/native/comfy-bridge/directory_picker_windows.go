//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const directoryPickerScript = `
Add-Type -AssemblyName System.Windows.Forms
$form = New-Object System.Windows.Forms.Form
$form.TopMost = $true
$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description = $env:COMFY_BRIDGE_PICKER_TITLE
if ($dialog.ShowDialog($form) -ne [System.Windows.Forms.DialogResult]::OK) {
    $form.Dispose()
    return
}
[Console]::Out.Write([System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($dialog.SelectedPath)))
$form.Dispose()
`

func pickDirectory(title string) (string, bool, error) {
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-STA", "-Command", directoryPickerScript)
	command.Env = append(os.Environ(), "COMFY_BRIDGE_PICKER_TITLE="+title)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", false, fmt.Errorf("打开 Windows 目录选择器失败：%s", detail)
	}
	path, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(output)))
	if err != nil {
		return "", false, fmt.Errorf("Windows 目录选择器返回值无效：%w", err)
	}
	selected := strings.TrimSpace(string(path))
	if selected == "" {
		return "", true, nil
	}
	stat, err := os.Stat(selected)
	if err != nil || !stat.IsDir() {
		return "", false, fmt.Errorf("Windows 目录选择器返回的路径无效：%s", selected)
	}
	return selected, false, nil
}
