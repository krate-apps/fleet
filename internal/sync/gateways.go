package sync

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/rtorrent"
)

const localRTorrentClientID = "local-rtorrent"

// GatewayEndpoint is a pollable remote BitTorrent client.
type GatewayEndpoint struct {
	ID       string
	Kind     gateways.Kind
	Name     string
	URL      string
	Username string
	Password string
	Enabled  bool
}

// GatewaySource supplies enabled gateway clients for multi-client torrent sync.
type GatewaySource interface {
	ListGateways(ctx context.Context) ([]GatewayEndpoint, error)
}

// SetGatewaySource attaches Settings → Clients as an additional torrent source.
func (e *Engine) SetGatewaySource(src GatewaySource) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.gateways = src
	e.mu.Unlock()
}

func (e *Engine) gatewaySource() GatewaySource {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.gateways
}

// torrentRowKey returns the unique cache/UI key for a torrent row.
func torrentRowKey(t rtorrent.Torrent) string {
	if id := strings.TrimSpace(t.ID); id != "" {
		return id
	}
	return strings.ToUpper(strings.TrimSpace(t.Hash))
}

func rowIDFor(hash, clientID string) string {
	return strings.ToUpper(strings.TrimSpace(hash)) + "@" + clientID
}

func withClient(t rtorrent.Torrent, clientID, kind, name string) rtorrent.Torrent {
	hash := strings.ToUpper(strings.TrimSpace(t.Hash))
	t.Hash = hash
	t.ClientID = clientID
	t.ClientKind = kind
	t.ClientName = name
	t.ID = rowIDFor(hash, clientID)
	return t
}

// mergeTorrentLists emits one row per client instance (same infohash may appear multiple times).
func mergeTorrentLists(primary []rtorrent.Torrent, remote []remoteBatch) []rtorrent.Torrent {
	out := make([]rtorrent.Torrent, 0, len(primary)+len(remote)*4)
	seen := make(map[string]struct{}, len(primary)+len(remote)*4)

	add := func(t rtorrent.Torrent) {
		key := torrentRowKey(t)
		if key == "" || key == "@" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}

	for _, t := range primary {
		add(withClient(t, localRTorrentClientID, string(gateways.KindRTorrent), "rTorrent"))
	}
	for _, batch := range remote {
		for _, row := range batch.Torrents {
			t := row.ToRTorrent()
			if t.Hash == "" {
				continue
			}
			name := batch.Name
			if name == "" {
				name = string(batch.Kind)
			}
			add(withClient(t, batch.ID, string(batch.Kind), name))
		}
	}
	return out
}

type remoteBatch struct {
	ID       string
	Kind     gateways.Kind
	Name     string
	Torrents []gateways.ListedTorrent
}

type gatewayPollResult struct {
	Batches  []remoteBatch
	Total    int
	OK       int
	Problems []ClientProblem
}

func (e *Engine) pollGateways(ctx context.Context) gatewayPollResult {
	src := e.gatewaySource()
	if src == nil {
		return gatewayPollResult{}
	}
	list, err := src.ListGateways(ctx)
	if err != nil {
		e.logger.Warn("gateway list failed", "err", err)
		return gatewayPollResult{
			Problems: []ClientProblem{{Name: "gateways", Message: err.Error()}},
		}
	}

	type result struct {
		batch remoteBatch
		err   error
		id    string
		kind  gateways.Kind
		name  string
	}
	ch := make(chan result, len(list))
	var wg sync.WaitGroup
	polled := 0
	for _, gw := range list {
		if !gw.Enabled {
			continue
		}
		// Primary engine already covers local rTorrent; skip duplicate registered rTorrent.
		if gw.Kind == gateways.KindRTorrent && e.client != nil {
			continue
		}
		gw := gw
		polled++
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
			defer cancel()
			rows, err := gateways.ListTorrents(cctx, gateways.ListInput{
				Kind:     gw.Kind,
				URL:      gw.URL,
				Username: gw.Username,
				Password: gw.Password,
			})
			ch <- result{
				batch: remoteBatch{ID: gw.ID, Kind: gw.Kind, Name: gw.Name, Torrents: rows},
				err:   err,
				id:    gw.ID,
				kind:  gw.Kind,
				name:  gw.Name,
			}
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	out := gatewayPollResult{Total: polled}
	for res := range ch {
		name := res.name
		if name == "" {
			name = string(res.kind)
		}
		if res.err != nil {
			log := e.logger
			if log == nil {
				log = slog.Default()
			}
			log.Warn("gateway torrent list failed", "kind", res.kind, "name", name, "err", res.err)
			out.Problems = append(out.Problems, ClientProblem{
				ID:      res.id,
				Name:    name,
				Kind:    string(res.kind),
				Message: res.err.Error(),
			})
			continue
		}
		out.OK++
		out.Batches = append(out.Batches, res.batch)
	}
	return out
}
