package rtorrent

import (
	"context"
	"sort"
	"strings"
	"time"
)

// MatrixProbe is a lab row for OPEN-02 (safe read-only RPC checks).
type MatrixProbe struct {
	Address       string            `json:"address"`
	Connected     bool              `json:"connected"`
	ClientVersion string            `json:"clientVersion,omitempty"`
	APIVersion    string            `json:"apiVersion,omitempty"`
	ProbedAt      string            `json:"probedAt"`
	Methods       map[string]string `json:"methods"` // method -> "ok" | error text | "listed"
	Groups        map[string]bool   `json:"groups"`  // matrix column keys
	Message       string            `json:"message,omitempty"`
}

// ProbeMatrix runs non-destructive method existence checks for the support matrix.
func (c *Client) ProbeMatrix(ctx context.Context) MatrixProbe {
	row := MatrixProbe{
		Address:  c.ep.String(),
		ProbedAt: time.Now().UTC().Format(time.RFC3339),
		Methods:  map[string]string{},
		Groups:   map[string]bool{},
	}

	caps := c.Probe(ctx)
	row.Connected = caps.Connected
	row.ClientVersion = caps.ClientVersion
	row.APIVersion = caps.APIVersion
	row.Message = caps.Message
	if !caps.Connected {
		return row
	}

	listed := map[string]struct{}{}
	if v, err := c.Call(ctx, "system.listMethods"); err == nil {
		row.Methods["system.listMethods"] = "ok"
		for _, item := range v.Array {
			name := strings.TrimSpace(item.AsString())
			if name != "" {
				listed[name] = struct{}{}
			}
		}
	} else {
		row.Methods["system.listMethods"] = err.Error()
	}

	markListed := func(name string) bool {
		_, ok := listed[name]
		if ok {
			row.Methods[name] = "listed"
		}
		return ok
	}

	// Live read-only calls where safe.
	live := []struct {
		group  string
		method string
		params []any
	}{
		{"list", "d.multicall2", []any{"", "main", "d.hash="}},
		{"list", "d.multicall", []any{"main", "d.hash="}},
		{"throttle", "throttle.global_down.rate", nil},
		{"throttle", "throttle.global_up.rate", nil},
		{"views", "view.list", nil},
		{"network", "network.listen.port", nil},
	}
	for _, ch := range live {
		if _, err := c.Call(ctx, ch.method, ch.params...); err != nil {
			row.Methods[ch.method] = err.Error()
			continue
		}
		row.Methods[ch.method] = "ok"
		row.Groups[ch.group] = true
	}

	// Action / detail methods: prefer listMethods (no side effects). Many modern
	// rTorrent builds lack system.methodExist.
	needed := []struct {
		group  string
		method string
		flag   string
	}{
		{"actions", "d.start", "start_stop_hash"},
		{"actions", "d.stop", "start_stop_hash"},
		{"actions", "d.check_hash", "start_stop_hash"},
		{"actions", "d.erase", "erase"},
		{"files", "f.multicall", "files"},
		{"trackers", "t.multicall", "trackers"},
		{"peers", "p.multicall", "peers"},
	}
	for _, n := range needed {
		if markListed(n.method) {
			row.Groups[n.group] = true
			row.Groups[n.flag] = true
		} else if _, ok := listed[n.method]; !ok && len(listed) > 0 {
			row.Methods[n.method] = "absent from system.listMethods"
		} else if len(listed) == 0 {
			// Fallback: unknown — leave unset for lab follow-up.
			if row.Methods[n.method] == "" {
				row.Methods[n.method] = "listMethods unavailable"
			}
		}
	}

	// start_stop_hash requires all three action methods.
	row.Groups["start_stop_hash"] = row.Methods["d.start"] == "listed" &&
		row.Methods["d.stop"] == "listed" &&
		row.Methods["d.check_hash"] == "listed"
	row.Groups["erase"] = row.Methods["d.erase"] == "listed"
	row.Groups["files"] = row.Methods["f.multicall"] == "listed"
	row.Groups["trackers"] = row.Methods["t.multicall"] == "listed"
	row.Groups["peers"] = row.Methods["p.multicall"] == "listed"
	row.Groups["multicall2"] = row.Methods["d.multicall2"] == "ok"

	keys := make([]string, 0, len(row.Methods))
	for k := range row.Methods {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(keys))
	for _, k := range keys {
		ordered[k] = row.Methods[k]
	}
	row.Methods = ordered
	return row
}
