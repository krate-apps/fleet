package logs

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// ClientRef is a gateway (or primary engine) log source candidate.
type ClientRef struct {
	ID          string
	Kind        string
	Name        string
	Enabled     bool
	JournalUnit string // optional override
	LogPath     string // optional file override
}

// Config wires aggregator discovery.
type Config struct {
	InstallPrefix   string
	RTorrentAddress string
	Clients         []ClientRef
	HomeDir         string
	Journal         JournalRunner
}

// Aggregator merges Fleet ring + journal/file client sources.
type Aggregator struct {
	ring *Ring
	cfg  Config
	user string
}

// NewAggregator constructs an aggregator around a Fleet ring.
func NewAggregator(ring *Ring, cfg Config) *Aggregator {
	if ring == nil {
		ring = NewRing(defaultRingSize)
	}
	home := cfg.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	cfg.HomeDir = home
	return &Aggregator{
		ring: ring,
		cfg:  cfg,
		user: InstanceUser(cfg.InstallPrefix),
	}
}

// Ring exposes the in-process Fleet buffer.
func (a *Aggregator) Ring() *Ring { return a.ring }

// SetClients refreshes gateway-derived sources (call after Settings changes).
func (a *Aggregator) SetClients(clients []ClientRef) {
	a.cfg.Clients = clients
}

// Sources lists available origins with health hints.
func (a *Aggregator) Sources(ctx context.Context) []Source {
	out := []Source{{
		ID:     "fleet",
		Kind:   "fleet",
		Name:   "Fleet",
		OK:     true,
		Detail: "in-process ring",
	}}
	if unit := DefaultUnitForKind("fleet", a.user); unit != "" {
		out = append(out, Source{
			ID:     "journal:" + unit,
			Kind:   "fleet",
			Name:   "Fleet (journal)",
			OK:     true,
			Detail: unit,
		})
	}
	hasRtorrentGateway := false
	for _, c := range a.cfg.Clients {
		if c.Enabled && strings.EqualFold(c.Kind, "rtorrent") {
			hasRtorrentGateway = true
			break
		}
	}
	if strings.TrimSpace(a.cfg.RTorrentAddress) != "" && !hasRtorrentGateway {
		unit := DefaultUnitForKind("rtorrent", a.user)
		out = append(out, Source{
			ID:     "journal:" + unit,
			Kind:   "rtorrent",
			Name:   "rTorrent",
			OK:     true,
			Detail: unit,
		})
	}
	for _, c := range a.cfg.Clients {
		if !c.Enabled {
			continue
		}
		src := a.clientSource(c)
		if src.ID == "" {
			continue
		}
		out = append(out, src)
	}
	return out
}

func (a *Aggregator) clientSource(c ClientRef) Source {
	name := c.Name
	if name == "" {
		name = c.Kind
	}
	if path := strings.TrimSpace(c.LogPath); path != "" {
		return Source{
			ID:     "gateway:" + c.ID,
			Kind:   c.Kind,
			Name:   name,
			OK:     true,
			Detail: path,
		}
	}
	if path := FirstExistingLogPath(DefaultLogPathsForKind(c.Kind, a.cfg.HomeDir), a.cfg.HomeDir); path != "" {
		unit := strings.TrimSpace(c.JournalUnit)
		if unit == "" {
			unit = DefaultUnitForKind(c.Kind, a.user)
		}
		detail := path
		if unit != "" {
			detail = unit + " + file"
		}
		return Source{
			ID:     "gateway:" + c.ID,
			Kind:   c.Kind,
			Name:   name,
			OK:     true,
			Detail: detail,
		}
	}
	unit := strings.TrimSpace(c.JournalUnit)
	if unit == "" {
		unit = DefaultUnitForKind(c.Kind, a.user)
	}
	if _, err := ValidateUnit(unit); err != nil {
		return Source{
			ID:     "gateway:" + c.ID,
			Kind:   c.Kind,
			Name:   name,
			OK:     false,
			Detail: err.Error(),
		}
	}
	return Source{
		ID:     "gateway:" + c.ID,
		Kind:   c.Kind,
		Name:   name,
		OK:     true,
		Detail: unit,
	}
}

// Query returns merged entries sorted ascending by timestamp.
func (a *Aggregator) Query(ctx context.Context, q Query) (sources []Source, entries []Entry, err error) {
	sources = a.Sources(ctx)
	limit := q.Limit
	if limit <= 0 {
		limit = 300
	}
	if limit > 1000 {
		limit = 1000
	}
	want := strings.TrimSpace(q.Source)
	level := strings.ToLower(strings.TrimSpace(q.Level))

	var all []Entry
	includeFleet := want == "" || want == "fleet" || strings.HasPrefix(want, "journal:fleet@")
	if includeFleet {
		all = append(all, a.ring.Snapshot(limit)...)
		if unit := DefaultUnitForKind("fleet", a.user); unit != "" && (want == "" || want == "fleet" || want == "journal:"+unit) {
			if je, jerr := ReadJournal(ctx, unit, limit, a.cfg.Journal); jerr == nil {
				all = append(all, je...)
			} else {
				all = append(all, diagnosticEntry("journal:"+unit, "fleet", "Fleet (journal)", jerr.Error()))
			}
		}
	}

	if strings.TrimSpace(a.cfg.RTorrentAddress) != "" {
		unit := DefaultUnitForKind("rtorrent", a.user)
		id := "journal:" + unit
		if want == "" || want == id || want == "rtorrent" {
			if je, jerr := ReadJournal(ctx, unit, limit, a.cfg.Journal); jerr == nil {
				all = append(all, je...)
			} else {
				all = append(all, diagnosticEntry(id, "rtorrent", "rTorrent", jerr.Error()))
			}
		}
	}

	for _, c := range a.cfg.Clients {
		if !c.Enabled {
			continue
		}
		sid := "gateway:" + c.ID
		if want != "" && want != sid && want != c.ID && want != c.Kind {
			continue
		}
		name := c.Name
		if name == "" {
			name = c.Kind
		}
		entries, err := a.readClientEntries(ctx, c, sid, name, limit)
		if err != nil {
			all = append(all, diagnosticEntry(sid, c.Kind, name, err.Error()))
			continue
		}
		all = append(all, entries...)
	}

	if level != "" {
		filtered := all[:0]
		for _, e := range all {
			if e.Level == level {
				filtered = append(filtered, e)
			}
		}
		all = filtered
	}

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].TS.Equal(all[j].TS) {
			return all[i].SourceID < all[j].SourceID
		}
		return all[i].TS.Before(all[j].TS)
	})
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	return sources, all, nil
}

func (a *Aggregator) readClientEntries(ctx context.Context, c ClientRef, sid, name string, limit int) ([]Entry, error) {
	if path := strings.TrimSpace(c.LogPath); path != "" {
		return ReadLogFile(path, a.cfg.HomeDir, sid, c.Kind, name, limit)
	}

	var journalEntries []Entry
	unit := strings.TrimSpace(c.JournalUnit)
	if unit == "" {
		unit = DefaultUnitForKind(c.Kind, a.user)
	}
	var journalErr error
	if unit != "" {
		je, jerr := ReadJournal(ctx, unit, limit, a.cfg.Journal)
		if jerr != nil {
			journalErr = jerr
		} else {
			for i := range je {
				je[i].SourceID = sid
				je[i].SourceKind = c.Kind
				je[i].SourceName = name
			}
			journalEntries = je
		}
	}

	// qBittorrent (and some daemons) log to files; journal may be empty after rotate.
	if path := FirstExistingLogPath(DefaultLogPathsForKind(c.Kind, a.cfg.HomeDir), a.cfg.HomeDir); path != "" {
		fe, ferr := ReadLogFile(path, a.cfg.HomeDir, sid, c.Kind, name, limit)
		if ferr == nil {
			if len(journalEntries) == 0 {
				return fe, nil
			}
			return append(journalEntries, fe...), nil
		}
		if len(journalEntries) == 0 && journalErr != nil {
			return nil, journalErr
		}
		if len(journalEntries) == 0 {
			return nil, ferr
		}
	}

	if len(journalEntries) > 0 {
		return journalEntries, nil
	}
	if journalErr != nil {
		return nil, journalErr
	}
	if unit == "" {
		return nil, fmt.Errorf("no journal unit or log file for %s", c.Kind)
	}
	return nil, nil
}

func diagnosticEntry(id, kind, name, msg string) Entry {
	return Entry{
		TS:         time.Now().UTC(),
		Level:      "warn",
		Message:    msg,
		SourceID:   id,
		SourceKind: kind,
		SourceName: name,
		Stream:     "diagnostic",
	}
}
