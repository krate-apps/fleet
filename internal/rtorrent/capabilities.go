package rtorrent

import (
	"context"
	"strings"
)

// Capabilities describes what this rTorrent build exposes to Fleet.
type Capabilities struct {
	Connected     bool     `json:"connected"`
	Engine        string   `json:"engine"`
	Address       string   `json:"address"`
	ClientVersion string   `json:"clientVersion,omitempty"`
	APIVersion    string   `json:"apiVersion,omitempty"`
	ListenPort    int64    `json:"listenPort,omitempty"`
	Capabilities  []string `json:"capabilities"`
	Message       string   `json:"message,omitempty"`
}

// Probe gathers version info and a capability list for the UI/API matrix.
func (c *Client) Probe(ctx context.Context) Capabilities {
	out := Capabilities{
		Engine:       "rtorrent",
		Address:      c.ep.String(),
		Capabilities: []string{},
	}

	clientVer, err := c.CallString(ctx, "system.client_version")
	if err != nil {
		out.Connected = false
		out.Message = err.Error()
		return out
	}
	out.Connected = true
	out.ClientVersion = clientVer

	if apiVer, err := c.CallString(ctx, "system.api_version"); err == nil {
		out.APIVersion = apiVer
	}

	caps := []string{
		"torrents.list",
		"torrents.rates",
		"engine.version",
	}

	// Feature probes: success means the method exists for this build.
	probes := []struct {
		method string
		cap    string
		params []any
	}{
		{method: "d.multicall2", cap: "torrents.multicall2", params: []any{"", "main", "d.hash="}},
		{method: "view.list", cap: "views.list", params: nil},
		{method: "network.listen.port", cap: "network.listen_port", params: nil},
		{method: "throttle.global_down.rate", cap: "throttle.rates", params: nil},
	}
	for _, p := range probes {
		v, err := c.Call(ctx, p.method, p.params...)
		if err != nil {
			continue
		}
		caps = append(caps, p.cap)
		if p.cap == "network.listen_port" {
			if n, err := v.AsInt(); err == nil {
				out.ListenPort = n
			}
		}
	}
	out.Capabilities = uniqueSorted(caps)
	return out
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
