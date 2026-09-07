//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func pickDirectory(title string) (string, bool, error) {
	if _, err := exec.LookPath("zenity"); err != nil {
		return "", false, errors.New("Linux 目录选择器需要安装 zenity")
	}
	command := exec.Command("zenity", "--file-selection", "--directory", "--title", title)
	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return "", true, nil
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", false, fmt.Errorf("打开 Linux 目录选择器失败：%s", detail)
	}
	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return "", true, nil
	}
	stat, err := os.Stat(selected)
	if err != nil || !stat.IsDir() {
		return "", false, fmt.Errorf("Linux 目录选择器返回的路径无效：%s", selected)
	}
	return selected, false, nil
}
