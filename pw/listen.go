package pw

import (
	"net"
	"strconv"
)

// developmentPortShift is how far past the configured port a development run
// looks for one it can bind. It is bounded so that a machine with a wide range
// already taken reports the port the developer configured, rather than serving
// on an address so far away that nothing pointed at the project reaches it.
const developmentPortShift = 10

// listenApplication binds the port the configuration named.
//
// A development run walks forward from it when it cannot be bound, the way a
// front-end development server does, and says on the way which port it settled
// on. Two projects open at once — or one leftover process from a loop that did
// not unwind — otherwise leave the second run with a bind failure, nothing
// served, and a port to hunt down before any work can continue. Development is
// the environment where a second copy of the same application is ordinary, and
// where nothing outside the machine has been told an address.
//
// Every other environment binds what it was told and fails. An address is a
// contract there: the health check, the reverse proxy, and the operator all go
// to the port the configuration names, and a deployment that quietly answers
// somewhere else is worse than one that does not start.
//
// The walk does not ask why the first bind failed. The address is a wildcard
// with a configured port, so what can go wrong is the port being held or being
// privileged, and both mean the same thing here: this port cannot be served.
// Telling them apart needs an errno the framework cannot compare portably —
// Windows reports a winsock number that syscall.EADDRINUSE does not match — and
// getting that wrong would leave the shift silently dead on a platform it was
// asked for. When nothing in range binds, the first failure is what the caller
// sees, because that is the port the developer asked about.
func listenApplication(config ServerConfig) (net.Listener, error) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(config.Port))
	if err == nil || !Development() {
		return listener, err
	}
	for port := config.Port + 1; port <= config.Port+developmentPortShift && port <= 65535; port++ {
		shifted, shiftErr := net.Listen("tcp", ":"+strconv.Itoa(port))
		if shiftErr != nil {
			continue
		}
		reportPortShift(config.Port, port, err)
		return shifted, nil
	}
	return nil, err
}

// reportPortShift says that this process is not on the port its configuration
// names.
//
// It is a warning rather than a line in the startup summary because the summary
// still reports the configured value beside the address actually bound, and a
// reader comparing the two deserves to be told which one moved and why. The
// environment is named as well: an unset APP_ENV resolves to development, so a
// deployment that forgot the variable can reach this, and the one thing it needs
// to know is that setting it restores the strict bind.
func reportPortShift(configured, bound int, cause error) {
	processLogger().Warn("the configured port could not be bound, so this development run moved to the next free one",
		Int("configured_port", configured),
		Int("port", bound),
		String("cause", cause.Error()),
		String("environment", Env()),
		String("effect", "only a development run shifts; every other environment fails instead of answering on a port nobody configured"),
	)
}
