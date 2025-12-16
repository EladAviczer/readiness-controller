package prober

import (
	"log"
	"os/exec"
	"strings"
)

type ExecProber struct {
	Command []string
}

func NewExecProber(cmdStr string) *ExecProber {
	parts := strings.Fields(cmdStr)
	return &ExecProber{Command: parts}
}

func (p *ExecProber) Check() bool {
	if len(p.Command) == 0 {
		return false
	}
	cmd := exec.Command(p.Command[0], p.Command[1:]...)
	err := cmd.Run()
	if err != nil {
		log.Printf("[Exec] Command failed: %v", err)
		return false
	}
	return true
}
