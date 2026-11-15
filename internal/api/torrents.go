package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/intake"
	"github.com/krate-apps/fleet/internal/rtorrent"
)

type actionsRequest struct {
	Action  string   `json:"action"`
	Hashes  []string `json:"hashes"`
	Targets []struct {
		Hash     string `json:"hash"`
		ClientID string `json:"clientId"`
	} `json:"targets"`
}

type addRequest struct {
	Magnets       []string `json:"magnets"`
	TorrentBase64 string   `json:"torrentBase64"`
	Start         *bool    `json:"start"`
	GatewayID     string   `json:"gatewayId"`
}

func (s *Server) handleTorrentActions(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	var req actionsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	action := rtorrent.Action(strings.ToLower(strings.TrimSpace(req.Action)))
	switch action {
	case rtorrent.ActionStart, rtorrent.ActionStop, rtorrent.ActionPause, rtorrent.ActionResume,
		rtorrent.ActionHash, rtorrent.ActionErase, rtorrent.ActionEraseData, rtorrent.ActionReannounce:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported action"})
		return
	}
	hashes := append([]string{}, req.Hashes...)
	for _, t := range req.Targets {
		h := strings.ToUpper(strings.TrimSpace(t.Hash))
		if h == "" {
			continue
		}
		if cid := strings.TrimSpace(t.ClientID); cid != "" {
			hashes = append(hashes, h+"@"+cid)
		} else {
			hashes = append(hashes, h)
		}
	}
	if len(hashes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hashes or targets required"})
		return
	}
	if len(hashes) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too many hashes"})
		return
	}

	results, err := s.engine.ApplyAction(r.Context(), action, hashes, s.cfg.RTorrent.DownloadRoots)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action":  action,
		"results": results,
	})
}

func (s *Server) handleAddTorrents(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	start := true
	if req.Start != nil {
		start = *req.Start
	}

	var data []byte
	switch {
	case len(req.Magnets) > 0:
	case req.TorrentBase64 != "":
		var err error
		data, err = base64.StdEncoding.DecodeString(req.TorrentBase64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid torrentBase64"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "magnets or torrentBase64 required"})
		return
	}

	gatewayID, err := intake.Add(r.Context(), s.store, s.engine, intake.Request{
		Magnets:   req.Magnets,
		Torrent:   data,
		Start:     start,
		GatewayID: req.GatewayID,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	out := map[string]any{"ok": true, "started": start}
	if gatewayID != "" {
		out["gatewayId"] = gatewayID
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (s *Server) handleTorrentFiles(w http.ResponseWriter, r *http.Request) {
	s.handleTorrentDetail(w, r, "files")
}

func (s *Server) handleTorrentTrackers(w http.ResponseWriter, r *http.Request) {
	s.handleTorrentDetail(w, r, "trackers")
}

func (s *Server) handleTorrentPeers(w http.ResponseWriter, r *http.Request) {
	s.handleTorrentDetail(w, r, "peers")
}

func (s *Server) handleTorrentChunks(w http.ResponseWriter, r *http.Request) {
	s.handleTorrentDetail(w, r, "chunks")
}

func (s *Server) handleTorrentDetail(w http.ResponseWriter, r *http.Request, kind string) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	rawID := r.PathValue("id")
	hash := strings.ToUpper(rawID)
	clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
	if i := strings.LastIndex(rawID, "@"); i > 0 {
		hash = strings.ToUpper(rawID[:i])
		if clientID == "" {
			clientID = rawID[i+1:]
		}
	}
	if hash == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing torrent id"})
		return
	}
	ctx := r.Context()
	switch kind {
	case "files":
		rows, err := s.engine.ListFilesOnClient(ctx, hash, clientID)
		if err != nil {
			writeDetailError(w, err, "files")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"hash": hash, "clientId": clientID, "files": rows})
	case "trackers":
		rows, err := s.engine.ListTrackersOnClient(ctx, hash, clientID)
		if err != nil {
			writeDetailError(w, err, "trackers")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"hash": hash, "clientId": clientID, "trackers": rows})
	case "peers":
		rows, err := s.engine.ListPeersOnClient(ctx, hash, clientID)
		if err != nil {
			writeDetailError(w, err, "peers")
			return
		}
		if s.geoip.Enabled() {
			for i := range rows {
				if cc := s.geoip.CountryISO(rows[i].Address); cc != "" {
					rows[i].Country = cc
				}
			}
		}
		if s.store != nil && len(rows) > 0 {
			addrs := make([]string, len(rows))
			for i, p := range rows {
				addrs[i] = p.Address
			}
			comments, err := s.store.ListPeerComments(addrs)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for i := range rows {
				if c := comments[rows[i].Address]; c != "" {
					rows[i].Comment = c
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"hash": hash, "clientId": clientID, "peers": rows})
	case "chunks":
		if clientID != "" && clientID != "local-rtorrent" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":   "unsupported on this client",
				"code":    "unsupported",
				"feature": "chunks",
			})
			return
		}
		m, err := s.engine.GetChunks(ctx, hash)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, m)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown detail"})
	}
}

func writeDetailError(w http.ResponseWriter, err error, feature string) {
	var un *gateways.UnsupportedError
	if errors.As(err, &un) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":      un.Error(),
			"code":       "unsupported",
			"feature":    feature,
			"clientKind": un.Kind,
		})
		return
	}
	if errors.Is(err, gateways.ErrUnsupported) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":   err.Error(),
			"code":    "unsupported",
			"feature": feature,
		})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

func (s *Server) handlePeerComment(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage unavailable"})
		return
	}
	var req struct {
		Address string `json:"address"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := s.store.SetPeerComment(req.Address, req.Comment); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"address": strings.TrimSpace(req.Address),
		"comment": strings.TrimSpace(req.Comment),
	})
}
