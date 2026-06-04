//go:build !windows

package runtimecfg

import (
	"os"
	"os/exec"
	"syscall"
)

func configureDaemonCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopProcess(process *os.Process) {
	_ = process.Signal(syscall.SIGTERM)
}
