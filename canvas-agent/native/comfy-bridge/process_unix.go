//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func configureComfyProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalZero() os.Signal {
	return syscall.Signal(0)
}

func terminateComfyProcess(process *os.Process) error {
	if err := syscall.Kill(-process.Pid, syscall.SIGTERM); err != nil {
		return process.Kill()
	}
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}
