//go:build windows

package runtimecfg

import (
	"os"
	"os/exec"
)

func configureDaemonCommand(cmd *exec.Cmd) {}

func stopProcess(process *os.Process) {
	_ = process.Kill()
}
