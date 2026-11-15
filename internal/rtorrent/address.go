package rtorrent

import (
	"fmt"
	"net"
	"strings"
)

// Endpoint describes how to reach rTorrent over SCGI.
type Endpoint struct {
	Network string // "unix" or "tcp"
	Address string // socket path or host:port
}

// ParseAddress accepts unix:///path/to.sock, /path/to.sock, or scgi://host:port.
func ParseAddress(raw string) (Endpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Endpoint{}, fmt.Errorf("rtorrent address is empty")
	}

	switch {
	case strings.HasPrefix(raw, "unix://"):
		path := strings.TrimPrefix(raw, "unix://")
		if path == "" {
			return Endpoint{}, fmt.Errorf("unix address missing path")
		}
		return Endpoint{Network: "unix", Address: path}, nil
	case strings.HasPrefix(raw, "scgi://"):
		hostport := strings.TrimPrefix(raw, "scgi://")
		if hostport == "" {
			return Endpoint{}, fmt.Errorf("scgi address missing host:port")
		}
		if _, _, err := net.SplitHostPort(hostport); err != nil {
			return Endpoint{}, fmt.Errorf("scgi address: %w", err)
		}
		return Endpoint{Network: "tcp", Address: hostport}, nil
	case strings.HasPrefix(raw, "/"):
		return Endpoint{Network: "unix", Address: raw}, nil
	default:
		return Endpoint{}, fmt.Errorf("unsupported rtorrent address %q (want unix://… or scgi://host:port)", raw)
	}
}

func (e Endpoint) String() string {
	if e.Network == "unix" {
		return "unix://" + e.Address
	}
	return "scgi://" + e.Address
}
