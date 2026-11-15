package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/rtorrent"
)

// ActionTarget identifies a torrent on a specific client (PD-GW-10).
type ActionTarget struct {
	Hash     string
	ClientID string
}

func splitTorrentID(id string) (hash, clientID string) {
	id = strings.TrimSpace(id)
	if i := strings.LastIndex(id, "@"); i > 0 {
		return strings.ToUpper(id[:i]), id[i+1:]
	}
	return strings.ToUpper(id), ""
}

func (e *Engine) clientIDForHash(hash string) string {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if hash == "" {
		return localRTorrentClientID
	}
	_, list, _ := e.Snapshot()
	var local, other string
	matches := 0
	for _, t := range list {
		if !strings.EqualFold(t.Hash, hash) {
			continue
		}
		matches++
		cid := strings.TrimSpace(t.ClientID)
		if cid == "" || cid == localRTorrentClientID {
			local = localRTorrentClientID
		} else {
			other = cid
		}
	}
	if local != "" {
		return local
	}
	if matches == 1 && other != "" {
		return other
	}
	return localRTorrentClientID
}

func (e *Engine) findGateway(ctx context.Context, clientID string) (GatewayEndpoint, error) {
	src := e.gatewaySource()
	if src == nil {
		return GatewayEndpoint{}, fmt.Errorf("gateways unavailable")
	}
	list, err := src.ListGateways(ctx)
	if err != nil {
		return GatewayEndpoint{}, err
	}
	for _, gw := range list {
		if gw.ID == clientID {
			if !gw.Enabled {
				return GatewayEndpoint{}, fmt.Errorf("gateway disabled")
			}
			return gw, nil
		}
	}
	return GatewayEndpoint{}, fmt.Errorf("gateway not found: %s", clientID)
}

// ApplyActionTargets runs a control action on per-client targets.
func (e *Engine) ApplyActionTargets(ctx context.Context, action rtorrent.Action, targets []ActionTarget, downloadRoots []string) ([]rtorrent.ActionResult, error) {
	out := make([]rtorrent.ActionResult, 0, len(targets))
	var localHashes []string
	for _, t := range targets {
		hash, cid := splitTorrentID(t.Hash)
		if t.ClientID != "" {
			cid = t.ClientID
		}
		if hash == "" {
			hash = strings.ToUpper(strings.TrimSpace(t.Hash))
		}
		if cid == "" {
			cid = e.clientIDForHash(hash)
		}
		if cid == "" || cid == localRTorrentClientID {
			localHashes = append(localHashes, hash)
			continue
		}
		gw, err := e.findGateway(ctx, cid)
		if err != nil {
			out = append(out, rtorrent.ActionResult{Hash: hash, ClientID: cid, OK: false, Message: err.Error()})
			continue
		}
		res := gateways.ApplyTorrentAction(ctx, gateways.ControlInput{
			Kind:     gw.Kind,
			URL:      gw.URL,
			Username: gw.Username,
			Password: gw.Password,
			Hash:     hash,
			Action:   action,
		})
		res.ClientID = cid
		out = append(out, res)
	}
	if len(localHashes) > 0 {
		localResults, err := e.applyLocalAction(ctx, action, localHashes, downloadRoots)
		if err != nil {
			return out, err
		}
		for i := range localResults {
			localResults[i].ClientID = localRTorrentClientID
		}
		out = append(out, localResults...)
	} else {
		e.Kick()
	}
	return out, nil
}

func (e *Engine) applyLocalAction(ctx context.Context, action rtorrent.Action, hashes []string, downloadRoots []string) ([]rtorrent.ActionResult, error) {
	var results []rtorrent.ActionResult
	err := e.withClient(func(c *rtorrent.Client) error {
		e.writeMu.Lock()
		defer e.writeMu.Unlock()
		results = c.ApplyAction(ctx, action, hashes, downloadRoots)
		e.Kick()
		return nil
	})
	return results, err
}

// SetRatesOnClient sets per-torrent up/down limits (bytes/sec; 0 = unlimited).
func (e *Engine) SetRatesOnClient(ctx context.Context, hash, clientID string, upBps, downBps int64) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = e.clientIDForHash(hash)
	}
	if upBps < 0 {
		upBps = 0
	}
	if downBps < 0 {
		downBps = 0
	}
	if clientID == "" || clientID == localRTorrentClientID {
		name := gateways.SyntheticThrottleName(upBps, downBps)
		if name != "" {
			if err := e.EnsureNamedThrottle(ctx, name, upBps, downBps); err != nil {
				return err
			}
		}
		if err := e.AssignTorrentThrottles(ctx, []string{hash}, name); err != nil {
			return err
		}
		e.Kick()
		return nil
	}
	gw, err := e.findGateway(ctx, clientID)
	if err != nil {
		return err
	}
	res := gateways.SetTorrentRates(ctx, gateways.RatesInput{
		Kind:     gw.Kind,
		URL:      gw.URL,
		Username: gw.Username,
		Password: gw.Password,
		Hash:     hash,
		UpBps:    upBps,
		DownBps:  downBps,
	})
	if !res.OK {
		return fmt.Errorf("%s", res.Message)
	}
	e.Kick()
	return nil
}

// SetLabelOnClient sets label on local rTorrent or a gateway client.
func (e *Engine) SetLabelOnClient(ctx context.Context, hash, clientID, label string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = e.clientIDForHash(hash)
	}
	if clientID == "" || clientID == localRTorrentClientID {
		return e.SetLabel(ctx, hash, label)
	}
	gw, err := e.findGateway(ctx, clientID)
	if err != nil {
		return err
	}
	res := gateways.SetTorrentLabel(ctx, gateways.LabelInput{
		Kind:     gw.Kind,
		URL:      gw.URL,
		Username: gw.Username,
		Password: gw.Password,
		Hash:     hash,
		Label:    label,
	})
	if !res.OK {
		return fmt.Errorf("%s", res.Message)
	}
	e.Kick()
	return nil
}

func (e *Engine) detailsInput(ctx context.Context, hash, clientID string) (gateways.DetailsInput, error) {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = e.clientIDForHash(hash)
	}
	if clientID == "" || clientID == localRTorrentClientID {
		return gateways.DetailsInput{}, fmt.Errorf("local")
	}
	gw, err := e.findGateway(ctx, clientID)
	if err != nil {
		return gateways.DetailsInput{}, err
	}
	return gateways.DetailsInput{
		Kind:     gw.Kind,
		URL:      gw.URL,
		Username: gw.Username,
		Password: gw.Password,
		Hash:     hash,
	}, nil
}

// ListFilesOnClient returns files for a torrent on the given client.
func (e *Engine) ListFilesOnClient(ctx context.Context, hash, clientID string) ([]rtorrent.FileEntry, error) {
	in, err := e.detailsInput(ctx, hash, clientID)
	if err != nil {
		if err.Error() == "local" {
			return e.ListFiles(ctx, hash)
		}
		return nil, err
	}
	return gateways.ListFiles(ctx, in)
}

// ListTrackersOnClient returns trackers for a torrent on the given client.
func (e *Engine) ListTrackersOnClient(ctx context.Context, hash, clientID string) ([]rtorrent.TrackerEntry, error) {
	in, err := e.detailsInput(ctx, hash, clientID)
	if err != nil {
		if err.Error() == "local" {
			return e.ListTrackers(ctx, hash)
		}
		return nil, err
	}
	return gateways.ListTrackers(ctx, in)
}

// ListPeersOnClient returns peers for a torrent on the given client.
func (e *Engine) ListPeersOnClient(ctx context.Context, hash, clientID string) ([]rtorrent.PeerEntry, error) {
	in, err := e.detailsInput(ctx, hash, clientID)
	if err != nil {
		if err.Error() == "local" {
			return e.ListPeers(ctx, hash)
		}
		return nil, err
	}
	return gateways.ListPeers(ctx, in)
}
