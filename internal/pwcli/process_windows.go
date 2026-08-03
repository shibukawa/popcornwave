//go:build windows

package pwcli

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// createNewProcessGroup is CREATE_NEW_PROCESS_GROUP, which makes the child the
// root of a group taskkill can end as a tree.
const createNewProcessGroup = 0x00000200

// ownProcessGroup puts a child at the root of a new process group, so the
// processes it starts can be ended with it. The developer loop runs the
// application through `go run`, which executes the compiled binary as a
// grandchild, and that grandchild is the process holding the port.
func ownProcessGroup(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= createNewProcessGroup
}

// signalProcessGroup ends the process tree. Windows has no signal another
// process can reliably deliver to a console application, so there is no
// graceful step here to distinguish from the forced one: both end the tree.
func signalProcessGroup(command *exec.Cmd, _ os.Signal) error {
	if command == nil || command.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
	return kill.Run()
}
