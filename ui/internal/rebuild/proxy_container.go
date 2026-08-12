package rebuild

import (
	"net"
	"net/url"
	"regexp"
	"runtime"
	"strings"
)

// hostDockerInternal is the Docker Desktop DNS name for the host machine
// from inside a container (Windows / macOS / modern Linux with host-gateway).
const hostDockerInternal = "host.docker.internal"

var localhostHost = regexp.MustCompile(`(?i)^(localhost|127\.0\.0\.1|\[::1\])$`)

// RewriteProxyForContainer rewrites a host-side proxy URL so it is reachable
// from inside a Linux container.
//
// Problem: GUI often sets http://localhost:8080 / 127.0.0.1:7890. Inside the
// container that points at the container itself → git clone fails with exit 128
// ("Connection refused" on localhost).
//
// Fix: replace localhost / 127.0.0.1 / ::1 with host.docker.internal.
// Callers should also pass DockerExtraHostArgs() so Linux engines resolve it.
func RewriteProxyForContainer(proxy string) string {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return ""
	}
	u, err := url.Parse(proxy)
	if err != nil || u.Host == "" {
		// best-effort string replace for non-URL forms host:port
		return rewriteHostPortProxy(proxy)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		// no port in host (unusual for proxy)
		host = u.Host
		port = ""
	}
	if localhostHost.MatchString(host) {
		if port != "" {
			u.Host = net.JoinHostPort(hostDockerInternal, port)
		} else {
			u.Host = hostDockerInternal
		}
		return u.String()
	}
	return proxy
}

func rewriteHostPortProxy(proxy string) string {
	// e.g. 127.0.0.1:7890 without scheme
	host, port, err := net.SplitHostPort(proxy)
	if err != nil {
		return proxy
	}
	if localhostHost.MatchString(host) {
		return net.JoinHostPort(hostDockerInternal, port)
	}
	return proxy
}

// DockerExtraHostArgs returns docker run flags so host.docker.internal works
// on Linux Engine (Docker Desktop already provides it on Windows/macOS).
func DockerExtraHostArgs() []string {
	// Safe on Desktop too: host-gateway maps to host IP.
	return []string{"--add-host=host.docker.internal:host-gateway"}
}

// ContainerNetworkHint documents the rewrite for logs/UI.
func ContainerNetworkHint(hostProxy, containerProxy string) string {
	if hostProxy == "" {
		return "容器未注入代理"
	}
	if hostProxy == containerProxy {
		return "容器代理=" + containerProxy
	}
	return "容器代理 " + hostProxy + " → " + containerProxy + "（localhost 已改为 host.docker.internal）"
}

// HostNeedsDockerInternal is true on typical Docker Desktop hosts.
func HostNeedsDockerInternal() bool {
	switch runtime.GOOS {
	case "windows", "darwin":
		return true
	default:
		return true // also recommend host-gateway on Linux
	}
}
