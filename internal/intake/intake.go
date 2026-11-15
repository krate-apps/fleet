package intake

import (
	"context"
	"fmt"
	"strings"

	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/sync"
)

// Request describes a magnet/.torrent add from the UI or an external indexer.
type Request struct {
	Magnets   []string
	Torrent   []byte
	Filename  string
	Start     bool
	GatewayID string // optional explicit target; empty uses the default client
}

// Add routes downloads to the selected/default gateway, falling back to the primary engine.
func Add(ctx context.Context, store *storage.Store, engine *sync.Engine, req Request) (gatewayID string, err error) {
	if len(req.Magnets) == 0 && len(req.Torrent) == 0 {
		return "", fmt.Errorf("magnets or torrent required")
	}

	targetID := strings.TrimSpace(req.GatewayID)
	var client storage.GatewayClient
	var haveClient bool

	if store != nil {
		if targetID != "" {
			c, err := store.GetGatewayClient(targetID)
			if err != nil {
				return "", err
			}
			client, haveClient = c, true
		} else if c, ok, err := store.GetDefaultGatewayClient(); err != nil {
			return "", err
		} else if ok {
			client, haveClient = c, true
			targetID = c.ID
		}
	}

	if haveClient {
		if !client.Enabled {
			return targetID, fmt.Errorf("default client %q is disabled", client.Name)
		}
		res := gateways.IntakeDownload(ctx, gateways.IntakeInput{
			Kind:     client.Kind,
			URL:      client.URL,
			Username: client.Username,
			Password: client.Password,
			Filename: req.Filename,
			Torrent:  req.Torrent,
			Magnets:  req.Magnets,
			Start:    req.Start,
		})
		if !res.OK {
			return targetID, fmt.Errorf("%s", res.Message)
		}
		return targetID, nil
	}

	if engine == nil {
		return "", fmt.Errorf("no default client and sync engine unavailable")
	}
	if len(req.Magnets) > 0 {
		if err := engine.AddMagnets(ctx, req.Magnets, req.Start); err != nil {
			return "", err
		}
	}
	if len(req.Torrent) > 0 {
		if err := engine.AddTorrentFile(ctx, req.Torrent, req.Start); err != nil {
			return "", err
		}
	}
	return "", nil
}
