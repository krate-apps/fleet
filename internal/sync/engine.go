package sync

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// Engine owns the single rTorrent poller and in-memory cache for an instance.
type Engine struct {
	client *rtorrent.Client
	logger *slog.Logger
	cache  *cache
	hub    *hub
	writeMu sync.Mutex
	kick   chan struct{}

	mu       sync.RWMutex
	gateways GatewaySource

	activeInterval  time.Duration
	idleInterval    time.Duration
	offlineInterval time.Duration
	probeEvery      time.Duration
}

// Options configures polling cadences.
type Options struct {
	ActiveInterval  time.Duration
	IdleInterval    time.Duration
	OfflineInterval time.Duration
	ProbeEvery      time.Duration
}

// NewEngine creates a synchronizer. client may be nil when no address is configured.
func NewEngine(client *rtorrent.Client, logger *slog.Logger, opts Options) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.ActiveInterval <= 0 {
		opts.ActiveInterval = time.Second
	}
	if opts.IdleInterval <= 0 {
		opts.IdleInterval = 3 * time.Second
	}
	if opts.OfflineInterval <= 0 {
		opts.OfflineInterval = 5 * time.Second
	}
	if opts.ProbeEvery <= 0 {
		opts.ProbeEvery = 30 * time.Second
	}
	e := &Engine{
		client:          client,
		logger:          logger,
		cache:           newCache(),
		hub:             newHub(),
		kick:            make(chan struct{}, 1),
		activeInterval:  opts.ActiveInterval,
		idleInterval:    opts.IdleInterval,
		offlineInterval: opts.OfflineInterval,
		probeEvery:      opts.ProbeEvery,
	}
	if client == nil {
		e.cache.setEngine(EngineStatus{
			Connected: false,
			Message:   "rtorrent address not configured",
		})
	}
	return e
}

// Run polls until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	if e.client == nil && e.gatewaySource() == nil {
		e.logger.Info("sync idle: no rtorrent client and no gateways")
		<-ctx.Done()
		return
	}
	if e.client == nil {
		e.logger.Info("sync: no rtorrent client; polling gateways only")
	}

	backoff := e.offlineInterval
	var lastProbe time.Time

	for {
		if ctx.Err() != nil {
			return
		}

		now := time.Now()
		var primary []rtorrent.Torrent
		rtOK := e.client != nil

		if e.client != nil {
			if lastProbe.IsZero() || now.Sub(lastProbe) >= e.probeEvery {
				caps := e.client.Probe(ctx)
				st := EngineStatus{
					Connected:     caps.Connected,
					Address:       caps.Address,
					ClientVersion: caps.ClientVersion,
					APIVersion:    caps.APIVersion,
					ListenPort:    caps.ListenPort,
					Capabilities:  caps.Capabilities,
					Message:       caps.Message,
					LastSync:      now.UTC().Format(time.RFC3339),
				}
				if ev, ok := e.cache.setEngine(st); ok {
					e.hub.publish(ev)
				}
				lastProbe = now
				if !caps.Connected {
					e.logger.Warn("rtorrent unavailable", "err", caps.Message)
					rtOK = false
				} else {
					backoff = e.offlineInterval
				}
			}

			if rtOK {
				list, err := e.client.ListTorrents(ctx)
				if err != nil {
					st := EngineStatus{
						Connected: false,
						Address:   e.client.Endpoint().String(),
						Message:   err.Error(),
						LastSync:  time.Now().UTC().Format(time.RFC3339),
					}
					if ev, ok := e.cache.setEngine(st); ok {
						e.hub.publish(ev)
					}
					e.logger.Warn("torrent sync failed", "err", err)
					rtOK = false
				} else {
					primary = list
					st := EngineStatus{
						Connected: true,
						Address:   e.client.Endpoint().String(),
						LastSync:  time.Now().UTC().Format(time.RFC3339),
					}
					_, _, prev := e.cache.snapshot()
					st.ClientVersion = prev.ClientVersion
					st.APIVersion = prev.APIVersion
					st.ListenPort = prev.ListenPort
					st.Capabilities = prev.Capabilities
					if ev, ok := e.cache.setEngine(st); ok {
						e.hub.publish(ev)
					}
				}
			}
		} else {
			// Gateways-only mode: mark engine connected when remotes respond.
			st := EngineStatus{
				Connected: true,
				Message:   "gateways only",
				LastSync:  now.UTC().Format(time.RFC3339),
			}
			if ev, ok := e.cache.setEngine(st); ok {
				e.hub.publish(ev)
			}
		}

		remote := e.pollGateways(ctx)
		combined := mergeTorrentLists(primary, remote.Batches)
		for _, ev := range e.cache.applyTorrents(combined) {
			e.hub.publish(ev)
		}

		_, _, prev := e.cache.snapshot()
		st := e.composeClientsStatus(prev, rtOK, remote)
		if ev, ok := e.cache.setEngine(st); ok {
			e.hub.publish(ev)
		}

		if st.ClientsConnected == 0 {
			if !sleep(ctx, backoff) {
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = e.offlineInterval

		interval := e.idleInterval
		if hasActivity(combined) {
			interval = e.activeInterval
		}
		if !sleepOrKick(ctx, interval, e.kick) {
			return
		}
	}
}

func (e *Engine) composeClientsStatus(prev EngineStatus, rtOK bool, remote gatewayPollResult) EngineStatus {
	st := prev
	st.LastSync = time.Now().UTC().Format(time.RFC3339)
	total := remote.Total
	ok := remote.OK
	problems := append([]ClientProblem(nil), remote.Problems...)
	if e.client != nil {
		total++
		if rtOK {
			ok++
		} else {
			msg := strings.TrimSpace(prev.Message)
			if msg == "" {
				msg = "unavailable"
			}
			problems = append([]ClientProblem{{
				ID:      localRTorrentClientID,
				Name:    "rTorrent",
				Kind:    "rtorrent",
				Message: msg,
			}}, problems...)
		}
	}
	st.ClientsTotal = total
	st.ClientsConnected = ok
	st.Problems = problems
	st.Connected = ok > 0
	if ok == 0 && strings.TrimSpace(st.Message) == "" {
		st.Message = "no torrent clients connected"
	}
	if ok > 0 {
		// Multi-client OK should not keep a stale single-engine error banner.
		if e.client == nil || rtOK {
			st.Message = ""
		}
	}
	return st
}

// Kick requests an immediate sync cycle after a mutating operation.
func (e *Engine) Kick() {
	if e == nil || e.kick == nil {
		return
	}
	select {
	case e.kick <- struct{}{}:
	default:
	}
}

func hasActivity(list []rtorrent.Torrent) bool {
	for _, t := range list {
		if t.DownRate > 0 || t.UpRate > 0 || t.Hashing {
			return true
		}
	}
	return false
}

func sleep(ctx context.Context, d time.Duration) bool {
	return sleepOrKick(ctx, d, nil)
}

func sleepOrKick(ctx context.Context, d time.Duration, kick <-chan struct{}) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	case <-kick:
		return true
	}
}

// Snapshot returns the current revision, torrents and engine status.
func (e *Engine) Snapshot() (uint64, []rtorrent.Torrent, EngineStatus) {
	return e.cache.snapshot()
}

// Subscribe receives live events. Caller must Unsubscribe.
func (e *Engine) Subscribe() chan Event {
	return e.hub.subscribe(64)
}

// Unsubscribe stops receiving events.
func (e *Engine) Unsubscribe(ch chan Event) {
	e.hub.unsubscribe(ch)
}

// Capabilities returns the latest engine status as an API-friendly payload.
func (e *Engine) Capabilities() map[string]any {
	_, _, st := e.cache.snapshot()
	return map[string]any{
		"manager":            "fleet",
		"engine":             "fleet",
		"connected":          st.Connected,
		"address":            st.Address,
		"clientVersion":      st.ClientVersion,
		"apiVersion":         st.APIVersion,
		"listenPort":         st.ListenPort,
		"capabilities":       st.Capabilities,
		"message":            st.Message,
		"lastSync":           st.LastSync,
		"clientsConnected":   st.ClientsConnected,
		"clientsTotal":       st.ClientsTotal,
		"problems":           st.Problems,
	}
}
