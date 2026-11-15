package sync

import "github.com/krate-apps/fleet/internal/rtorrent"

// EventType enumerates cache change kinds pushed to WebSocket clients.
type EventType string

const (
	EventSnapshot EventType = "snapshot"
	EventAdd      EventType = "add"
	EventUpdate   EventType = "update"
	EventRemove   EventType = "remove"
	EventEngine   EventType = "engine"
)

// Event is a single delta or snapshot frame for clients.
type Event struct {
	Type     EventType          `json:"type"`
	Revision uint64             `json:"revision"`
	Torrent  *rtorrent.Torrent  `json:"torrent,omitempty"`
	Torrents []rtorrent.Torrent `json:"torrents,omitempty"`
	Hash     string             `json:"hash,omitempty"`
	Engine   *EngineStatus      `json:"engine,omitempty"`
}

// ClientProblem describes a torrent client that failed the latest sync poll.
type ClientProblem struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Kind    string `json:"kind,omitempty"`
	Message string `json:"message,omitempty"`
}

// EngineStatus is the connection health exposed to the UI.
type EngineStatus struct {
	Connected     bool     `json:"connected"`
	Address       string   `json:"address,omitempty"`
	ClientVersion string   `json:"clientVersion,omitempty"`
	APIVersion    string   `json:"apiVersion,omitempty"`
	ListenPort    int64    `json:"listenPort,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Message       string   `json:"message,omitempty"`
	LastSync      string   `json:"lastSync,omitempty"`
	// Multi-client summary for the status bar (local rTorrent + gateways).
	ClientsConnected int             `json:"clientsConnected"`
	ClientsTotal     int             `json:"clientsTotal"`
	Problems         []ClientProblem `json:"problems,omitempty"`
}
