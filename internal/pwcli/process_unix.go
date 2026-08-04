//go:build !windows

package pwcli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// ownProcessGroup puts a child into a process group of its own, so a signal can
// reach everything that child starts. The developer loop runs the application
// through `go run`, which compiles and then executes the binary as a grandchild:
// signalling only the child leaves the process actually holding the port behind,
// and a kill is not forwarded at all.
//
// A group of its own also means the child no longer receives the terminal's
// Ctrl-C. That is deliberate. pw already handles the interrupt and stops its
// children in a known order, and a child that also got the signal directly would
// be racing that.
func ownProcessGroup(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
}

// signalProcessGroup delivers a signal to the whole group. A child that never
// got a group of its own still gets the signal directly, so this is safe for a
// command started without ownProcessGroup.
func signalProcessGroup(command *exec.Cmd, signal os.Signal) error {
	if command == nil || command.Process == nil {
		return nil
	}
	number, ok := signal.(syscall.Signal)
	if !ok {
		return command.Process.Signal(signal)
	}
	// A negative pid addresses the group. It fails when the child was never
	// given one, which is the case the process-level signal below covers.
	if err := syscall.Kill(-command.Process.Pid, number); err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return command.Process.Signal(signal)
}
