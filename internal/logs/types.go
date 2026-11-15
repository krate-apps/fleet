package logs

import "time"

// Entry is one aggregated log line with a stable source tag.
type Entry struct {
	TS          time.Time         `json:"ts"`
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	Attrs       map[string]string `json:"attrs,omitempty"`
	SourceID    string            `json:"sourceId"`
	SourceKind  string            `json:"sourceKind"`
	SourceName  string            `json:"sourceName"`
	Stream      string            `json:"stream,omitempty"` // ring | journal | file
}

// Source describes an available log origin for the UI filter chips.
type Source struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// Query filters aggregated log retrieval.
type Query struct {
	Source string // empty = all; "fleet" or gateway id / source id
	Level  string // empty = all; debug|info|warn|error
	Limit  int
}
