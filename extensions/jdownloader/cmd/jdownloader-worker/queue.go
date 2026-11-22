package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// QueueItem is the shared RPC shape for download-connector queues.
type QueueItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	BytesDone  int64  `json:"bytesDone"`
	BytesTotal int64  `json:"bytesTotal"`
	Speed      int64  `json:"speed"`
	EtaSec     int64  `json:"etaSec"`
	Kind       string `json:"kind"` // package | link
	Enabled    bool   `json:"enabled"`
}

type myjdPackage struct {
	UUID       any     `json:"uuid"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	BytesLoaded int64  `json:"bytesLoaded"`
	BytesTotal int64   `json:"bytesTotal"`
	Speed      float64 `json:"speed"`
	Eta        int64   `json:"eta"`
	Enabled    bool    `json:"enabled"`
	Finished   bool    `json:"finished"`
	Running    bool    `json:"running"`
}

func normalizeMyJDPackages(raw json.RawMessage) ([]QueueItem, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []QueueItem{}, nil
	}
	var pkgs []myjdPackage
	if err := json.Unmarshal(raw, &pkgs); err != nil {
		return nil, fmt.Errorf("queryPackages decode: %w", err)
	}
	out := make([]QueueItem, 0, len(pkgs))
	for _, p := range pkgs {
		id := anyToID(p.UUID)
		if id == "" {
			continue
		}
		status := strings.TrimSpace(p.Status)
		if status == "" {
			switch {
			case p.Finished:
				status = "finished"
			case p.Running:
				status = "running"
			case !p.Enabled:
				status = "disabled"
			default:
				status = "idle"
			}
		}
		eta := p.Eta
		if eta < 0 {
			eta = 0
		}
		out = append(out, QueueItem{
			ID:         id,
			Name:       p.Name,
			Status:     status,
			BytesDone:  p.BytesLoaded,
			BytesTotal: p.BytesTotal,
			Speed:      int64(p.Speed),
			EtaSec:     eta,
			Kind:       "package",
			Enabled:    p.Enabled,
		})
	}
	return out, nil
}

func anyToID(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func parseIDList(raw any) []any {
	switch v := raw.(type) {
	case nil:
		return []any{}
	case []any:
		out := make([]any, 0, len(v))
		for _, x := range v {
			if s := strings.TrimSpace(fmt.Sprint(x)); s != "" {
				if n, err := strconv.ParseInt(s, 10, 64); err == nil {
					out = append(out, n)
				} else {
					out = append(out, s)
				}
			}
		}
		return out
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return []any{}
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return []any{n}
		}
		return []any{s}
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			return []any{}
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return []any{n}
		}
		return []any{s}
	}
}
