package sync

import (
	"sync"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

type cache struct {
	mu       sync.RWMutex
	byKey    map[string]rtorrent.Torrent
	revision uint64
	engine   EngineStatus
}

func newCache() *cache {
	return &cache{byKey: make(map[string]rtorrent.Torrent)}
}

func (c *cache) snapshot() (uint64, []rtorrent.Torrent, EngineStatus) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]rtorrent.Torrent, 0, len(c.byKey))
	for _, t := range c.byKey {
		out = append(out, t)
	}
	return c.revision, out, c.engine
}

func (c *cache) setEngine(st EngineStatus) (Event, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := c.engine.Connected != st.Connected ||
		c.engine.Message != st.Message ||
		c.engine.ClientVersion != st.ClientVersion ||
		c.engine.ListenPort != st.ListenPort ||
		c.engine.ClientsConnected != st.ClientsConnected ||
		c.engine.ClientsTotal != st.ClientsTotal ||
		!clientProblemsEqual(c.engine.Problems, st.Problems)
	c.engine = st
	if !changed {
		return Event{}, false
	}
	c.revision++
	eng := st
	return Event{Type: EventEngine, Revision: c.revision, Engine: &eng}, true
}

func clientProblemsEqual(a, b []ClientProblem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Message != b[i].Message {
			return false
		}
	}
	return true
}

func (c *cache) applyTorrents(list []rtorrent.Torrent) []Event {
	c.mu.Lock()
	defer c.mu.Unlock()

	seen := make(map[string]struct{}, len(list))
	var events []Event

	for _, t := range list {
		key := torrentRowKey(t)
		seen[key] = struct{}{}
		prev, ok := c.byKey[key]
		if !ok {
			c.revision++
			cp := t
			c.byKey[key] = t
			events = append(events, Event{Type: EventAdd, Revision: c.revision, Torrent: &cp, Hash: key})
			continue
		}
		if torrentEqual(prev, t) {
			continue
		}
		c.revision++
		cp := t
		c.byKey[key] = t
		events = append(events, Event{Type: EventUpdate, Revision: c.revision, Torrent: &cp, Hash: key})
	}

	for key := range c.byKey {
		if _, ok := seen[key]; ok {
			continue
		}
		c.revision++
		delete(c.byKey, key)
		events = append(events, Event{Type: EventRemove, Revision: c.revision, Hash: key})
	}
	return events
}

func torrentEqual(a, b rtorrent.Torrent) bool {
	return a.Hash == b.Hash &&
		a.ID == b.ID &&
		a.ClientID == b.ClientID &&
		a.ClientKind == b.ClientKind &&
		a.ClientName == b.ClientName &&
		a.Name == b.Name &&
		a.SizeBytes == b.SizeBytes &&
		a.BytesDone == b.BytesDone &&
		a.LeftBytes == b.LeftBytes &&
		a.Complete == b.Complete &&
		a.Active == b.Active &&
		a.Open == b.Open &&
		a.Hashing == b.Hashing &&
		a.DownRate == b.DownRate &&
		a.UpRate == b.UpRate &&
		a.DownTotal == b.DownTotal &&
		a.UpTotal == b.UpTotal &&
		a.Ratio == b.Ratio &&
		a.Priority == b.Priority &&
		a.Directory == b.Directory &&
		a.Label == b.Label &&
		a.State == b.State &&
		a.ETASeconds == b.ETASeconds &&
		a.UploadETASeconds == b.UploadETASeconds &&
		a.StartedAt == b.StartedAt &&
		a.FinishedAt == b.FinishedAt &&
		a.SeedsComplete == b.SeedsComplete &&
		a.PeersConnected == b.PeersConnected
}
