//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func configureComfyProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func signalZero() os.Signal {
	return syscall.Signal(0)
}

func terminateComfyProcess(process *os.Process) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(process.Pid), "/T", "/F").Run()
}
