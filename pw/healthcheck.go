package pw

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// healthcheckCommandName is reserved for the framework so a shell-less
// container image can use its own binary as the HEALTHCHECK probe:
//
//	HEALTHCHECK CMD ["/app", "healthcheck"]
//
// The probe reads the same configuration sources as the server, so the port
// and endpoint path never need to be repeated in the Dockerfile.
const healthcheckCommandName = "healthcheck"

type healthcheckOptions struct {
	ready   bool
	timeout time.Duration
}

// defaultHealthcheckTimeout stays well under Docker's 30s HEALTHCHECK timeout
// default, so the probe reports a hung listener instead of being killed and
// reported as nothing.
const defaultHealthcheckTimeout = 3 * time.Second

func parseHealthcheckArgs(args []string) (frameworkAction, error) {
	options := healthcheckOptions{timeout: defaultHealthcheckTimeout}
	setTimeout := func(value string) error {
		timeout, err := time.ParseDuration(value)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("popcornwave: healthcheck --timeout needs a positive duration such as 3s, got %q", value)
		}
		options.timeout = timeout
		return nil
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--ready":
			options.ready = true
		case arg == "--timeout":
			if index+1 >= len(args) {
				return frameworkAction{}, fmt.Errorf("popcornwave: healthcheck --timeout needs a duration such as 3s")
			}
			index++
			if err := setTimeout(args[index]); err != nil {
				return frameworkAction{}, err
			}
		case strings.HasPrefix(arg, "--timeout="):
			if err := setTimeout(strings.TrimPrefix(arg, "--timeout=")); err != nil {
				return frameworkAction{}, err
			}
		default:
			return frameworkAction{}, fmt.Errorf("popcornwave: healthcheck does not accept %q; its options are --ready and --timeout", arg)
		}
	}
	return frameworkAction{kind: frameworkActionHealthcheck, healthcheck: options}, nil
}

// runHealthcheckProbe issues one GET against the configured health or
// readiness endpoint and reports the result through the exit status: the error
// path is the unhealthy verdict. Output stays to one short line because Docker
// retains only the first 4096 bytes in inspect, and per the operational
// endpoint rules it never includes configuration detail beyond the key an
// operator must set.
func runHealthcheckProbe(options healthcheckOptions) error {
	server := Config[ServerConfig](nil)
	path, key := server.Health, "server.health"
	if options.ready {
		path, key = server.Readiness, "server.readiness"
	}
	if path == "" {
		return fmt.Errorf("popcornwave: healthcheck needs %s configured, and it is unset", key)
	}
	if server.Port <= 0 {
		return fmt.Errorf("popcornwave: healthcheck cannot locate a listener on server.port %d; a container needs a fixed port", server.Port)
	}
	status, err := probeOperationalEndpoint(server.Port, path, options.timeout)
	if err != nil {
		return fmt.Errorf("popcornwave: healthcheck GET %s: %w", path, err)
	}
	if status < 200 || status > 299 {
		// A redirect is a failure too: the probe follows nothing, because the
		// operational endpoints answer in place and anything else means the
		// request did not reach them.
		return fmt.Errorf("popcornwave: healthcheck GET %s answered %d", path, status)
	}
	fmt.Fprintf(os.Stdout, "ok: GET %s %d\n", path, status)
	return nil
}

// probeOperationalEndpoint speaks HTTP/1.1 over its own dialed connection
// instead of using http.Client, because these are the primitives TinyGo
// supports — the same request.Write plus http.ReadResponse pairing the
// tinygodriver transport uses — and one code path serves both toolchains. The
// timeout bounds the whole probe: dial, write, and response share one
// deadline.
func probeOperationalEndpoint(port int, path string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, err
	}
	request, err := http.NewRequest(http.MethodGet, "http://"+address+path, nil)
	if err != nil {
		return 0, err
	}
	request.Close = true
	if err := request.Write(conn); err != nil {
		return 0, err
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}
