package pwconfig

import (
	"fmt"
	"strconv"
	"strings"
)

// HostingPort resolves the assigned listener port used by HTTP-to-process
// function hosts. PORT is already part of ServerConfig binding; these aliases
// are stronger because their host sends traffic to the assigned port.
func HostingPort(configured int, lookup func(string) (string, bool)) (int, error) {
	for _, name := range []string{"FUNCTIONS_CUSTOMHANDLER_PORT", "AWS_LWA_PORT"} {
		raw, exists := lookup(name)
		if !exists || strings.TrimSpace(raw) == "" {
			continue
		}
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("popcornweb: invalid %s %q: use a port from 1 through 65535", name, raw)
		}
		return port, nil
	}
	return configured, nil
}
