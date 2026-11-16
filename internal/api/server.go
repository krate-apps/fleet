package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/krate-apps/fleet/internal/auth"
	"github.com/krate-apps/fleet/internal/buckets"
	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/extensions"
	"github.com/krate-apps/fleet/internal/geoip"
	"github.com/krate-apps/fleet/internal/jobs"
	"github.com/krate-apps/fleet/internal/logs"
	"github.com/krate-apps/fleet/internal/share"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/sync"
	"github.com/krate-apps/fleet/internal/themes"
	"github.com/krate-apps/fleet/internal/tools"
	"github.com/krate-apps/fleet/internal/updates"
	"github.com/krate-apps/fleet/internal/version"
)

// Server is the Fleet HTTP API and static UI host.
type Server struct {
	cfg     config.Config
	logger  *slog.Logger
	mux     *http.ServeMux
	webFS   fs.FS
	engine  *sync.Engine
	tools   *tools.Runner
	jobs    *jobs.Manager
	ext     *extensions.Host
	buckets *buckets.Registry
	auth    *auth.Service
	updates *updates.Manager
	geoip   *geoip.DB
	store   *storage.Store
	shares  *share.Store
	logs    *logs.Aggregator
	themes  *themes.Store
}

// Options wires optional M3/M4/M5 services into the API server.
type Options struct {
	Jobs       *jobs.Manager
	Extensions *extensions.Host
	Buckets    *buckets.Registry
	Auth       *auth.Service
	Updates    *updates.Manager
	GeoIP      *geoip.DB
	Store      *storage.Store
	Shares     *share.Store
	Logs       *logs.Aggregator
	Themes     *themes.Store
}

// New constructs an API server. webFS may be nil when the UI is not embedded yet.
func New(cfg config.Config, logger *slog.Logger, webFS fs.FS, engine *sync.Engine, opts ...Options) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	s := &Server{
		cfg:    cfg,
		logger: logger,
		mux:    http.NewServeMux(),
		webFS:  webFS,
		engine: engine,
		tools: &tools.Runner{
			MktorrentPath:   cfg.Tools.Mktorrent,
			DumptorrentPath: cfg.Tools.Dumptorrent,
			MediainfoPath:   cfg.Tools.Mediainfo,
			FfmpegPath:      cfg.Tools.Ffmpeg,
			SoxPath:         cfg.Tools.Sox,
			AllowedRoots:    append([]string(nil), cfg.RTorrent.DownloadRoots...),
			ArtifactsDir:    filepath.Join(cfg.DataDir, "artifacts"),
		},
		jobs:    o.Jobs,
		ext:     o.Extensions,
		buckets: o.Buckets,
		auth:    o.Auth,
		updates: o.Updates,
		geoip:   o.GeoIP,
		store:   o.Store,
		shares:  o.Shares,
		logs:    o.Logs,
		themes:  o.Themes,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/version", s.handleVersion)
	s.mux.HandleFunc("GET /api/v1/themes", s.handleThemesList)
	s.mux.HandleFunc("GET /api/v1/themes/{id}/css", s.handleThemeCSS)
	s.mux.HandleFunc("POST /api/v1/themes/install", s.handleInstallTheme)
	s.mux.HandleFunc("POST /api/v1/themes/uninstall", s.handleUninstallTheme)
	s.mux.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("POST /api/v1/auth/setup", s.handleAuthSetup)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuthLogout)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.handleAuthMe)
	s.mux.HandleFunc("GET /api/v1/engine/capabilities", s.handleCapabilities)
	s.mux.HandleFunc("GET /api/v1/system/status", s.handleSystemStatus)
	s.mux.HandleFunc("GET /api/v1/torrents", s.handleTorrents)
	s.mux.HandleFunc("GET /api/v1/feeds", s.handleFeedsIndex)
	s.mux.HandleFunc("GET /api/v1/feeds/{mode}", s.handleFeedXML)
	s.mux.HandleFunc("GET /api/v1/settings/feeds", s.handleGetFeedsSettings)
	s.mux.HandleFunc("PUT /api/v1/settings/feeds", s.handlePutFeedsSettings)
	s.mux.HandleFunc("POST /api/v1/torrents", s.handleAddTorrents)
	s.mux.HandleFunc("POST /api/v1/torrents/actions", s.handleTorrentActions)
	s.mux.HandleFunc("GET /api/v1/torrents/{id}/files", s.handleTorrentFiles)
	s.mux.HandleFunc("GET /api/v1/torrents/{id}/files/{index}/content", s.handleTorrentFileContent)
	s.mux.HandleFunc("GET /api/v1/torrents/{id}/trackers", s.handleTorrentTrackers)
	s.mux.HandleFunc("GET /api/v1/torrents/{id}/peers", s.handleTorrentPeers)
	s.mux.HandleFunc("PUT /api/v1/peers/comment", s.handlePeerComment)
	s.mux.HandleFunc("GET /api/v1/torrents/{id}/chunks", s.handleTorrentChunks)
	s.mux.HandleFunc("GET /api/v1/torrents/{id}/source", s.handleTorrentSource)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/label", s.handleSetLabel)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/name", s.handleSetName)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/priority", s.handleSetTorrentPriority)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/files/priority", s.handleSetFilePriorities)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/trackers", s.handleAddTracker)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/trackers/enable", s.handleSetTrackerEnabled)
	s.mux.HandleFunc("POST /api/v1/torrents/{id}/trackers/remove", s.handleRemoveTracker)
	s.mux.HandleFunc("GET /api/v1/fs/roots", s.handleFSRoots)
	s.mux.HandleFunc("GET /api/v1/fs/list", s.handleFSList)
	s.mux.HandleFunc("POST /api/v1/fs/mkdir", s.handleFSMkdir)
	s.mux.HandleFunc("POST /api/v1/fs/rename", s.handleFSRename)
	s.mux.HandleFunc("POST /api/v1/fs/move", s.handleFSMove)
	s.mux.HandleFunc("POST /api/v1/fs/copy", s.handleFSCopy)
	s.mux.HandleFunc("POST /api/v1/fs/remove", s.handleFSRemove)
	s.mux.HandleFunc("GET /api/v1/fs/text", s.handleFSText)
	s.mux.HandleFunc("GET /api/v1/fs/content", s.handleFSContent)
	s.mux.HandleFunc("POST /api/v1/fs/checksum", s.handleFSChecksum)
	s.mux.HandleFunc("GET /api/v1/share/{token}", s.handleShareGet)
	s.mux.HandleFunc("GET /api/v1/shares", s.handleShareList)
	s.mux.HandleFunc("POST /api/v1/shares", s.handleShareCreate)
	s.mux.HandleFunc("DELETE /api/v1/shares/{token}", s.handleShareDelete)
	s.mux.HandleFunc("GET /api/v1/tools/status", s.handleToolsStatus)
	s.mux.HandleFunc("POST /api/v1/tools/dumptorrent", s.handleDumpTorrent)
	s.mux.HandleFunc("POST /api/v1/tools/mktorrent", s.handleCreateTorrent)
	s.mux.HandleFunc("POST /api/v1/tools/mediainfo", s.handleMediaInfo)
	s.mux.HandleFunc("GET /api/v1/jobs", s.handleListJobs)
	s.mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", s.handleCancelJob)
	s.mux.HandleFunc("GET /api/v1/jobs/{id}/download", s.handleJobDownload)
	s.mux.HandleFunc("GET /api/v1/extensions", s.handleListExtensions)
	s.mux.HandleFunc("POST /api/v1/extensions/reload-workers", s.handleReloadExtensionWorkers)
	s.mux.HandleFunc("GET /api/v1/extensions/{id}", s.handleGetExtension)
	s.mux.HandleFunc("POST /api/v1/extensions/{id}/activate", s.handleActivateExtension)
	s.mux.HandleFunc("POST /api/v1/extensions/{id}/deactivate", s.handleDeactivateExtension)
	s.mux.HandleFunc("POST /api/v1/extensions/{id}/consent", s.handleExtensionConsent)
	s.mux.HandleFunc("POST /api/v1/extensions/{id}/rpc", s.handleExtensionRPC)
	s.mux.HandleFunc("GET /api/v1/extensions/{id}/storage/{key}", s.handleExtensionStorageGet)
	s.mux.HandleFunc("PUT /api/v1/extensions/{id}/storage/{key}", s.handleExtensionStoragePut)
	s.mux.HandleFunc("POST /api/v1/extensions/install", s.handleInstallExtension)
	s.mux.HandleFunc("DELETE /api/v1/extensions/{id}/install", s.handleUninstallExtension)
	s.mux.HandleFunc("POST /api/v1/hooks/{id}", s.handleExtensionHook)
	s.mux.HandleFunc("GET /api/v1/buckets", s.handleListBuckets)
	s.mux.HandleFunc("POST /api/v1/buckets", s.handleAddBucket)
	s.mux.HandleFunc("DELETE /api/v1/buckets/{id}", s.handleRemoveBucket)
	s.mux.HandleFunc("POST /api/v1/buckets/{id}/enable", s.handleSetBucketEnabled)
	s.mux.HandleFunc("GET /api/v1/buckets/catalog", s.handleBucketCatalog)
	s.mux.HandleFunc("GET /api/v1/buckets/themes-catalog", s.handleThemesCatalog)
	s.mux.HandleFunc("GET /api/v1/updates", s.handleUpdatesStatus)
	s.mux.HandleFunc("PUT /api/v1/updates/channel", s.handleUpdatesSetChannel)
	s.mux.HandleFunc("GET /api/v1/updates/available", s.handleUpdatesAvailable)
	s.mux.HandleFunc("GET /api/v1/updates/installed", s.handleUpdatesInstalled)
	s.mux.HandleFunc("POST /api/v1/updates/prepare", s.handleUpdatesPrepare)
	s.mux.HandleFunc("POST /api/v1/updates/apply", s.handleUpdatesApply)
	s.mux.HandleFunc("POST /api/v1/updates/switch", s.handleUpdatesSwitch)
	s.mux.HandleFunc("POST /api/v1/updates/backup", s.handleUpdatesBackup)
	s.mux.HandleFunc("GET /api/v1/gateways", s.handleListGateways)
	s.mux.HandleFunc("POST /api/v1/gateways", s.handleUpsertGateway)
	s.mux.HandleFunc("PUT /api/v1/gateways/{id}", s.handleUpsertGateway)
	s.mux.HandleFunc("DELETE /api/v1/gateways/{id}", s.handleDeleteGateway)
	s.mux.HandleFunc("POST /api/v1/gateways/{id}/default", s.handleSetDefaultGateway)
	s.mux.HandleFunc("DELETE /api/v1/gateways/default", s.handleClearDefaultGateway)
	s.mux.HandleFunc("POST /api/v1/gateways/{id}/probe", s.handleProbeGateway)
	s.mux.HandleFunc("POST /api/v1/gateways/probe", s.handleProbeGatewayDraft)
	s.mux.HandleFunc("GET /api/v1/logs", s.handleLogs)
	s.mux.HandleFunc("GET /api/v1/events/ws", s.handleEventsWS)

	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		s.mux.Handle("GET /", spaHandler(s.webFS, fileServer))
	} else {
		s.mux.HandleFunc("GET /", s.handleNoUI)
	}
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.withLogging(s.withAuth(s.mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "fleet",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"builtAt": version.BuiltAt,
	})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, _ *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"engine":       "rtorrent",
			"connected":    false,
			"address":      s.cfg.RTorrent.Address,
			"capabilities": []string{},
			"message":      "sync engine not started",
			"geoip":        s.geoip.Enabled(),
		})
		return
	}
	payload := s.engine.Capabilities()
	payload["geoip"] = s.geoip.Enabled()
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleTorrents(w http.ResponseWriter, _ *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync engine unavailable"})
		return
	}
	rev, torrents, engine := s.engine.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"revision": rev,
		"torrents": torrents,
		"engine":   engine,
	})
}

func (s *Server) handleEventsWS(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		http.Error(w, "sync engine unavailable", http.StatusServiceUnavailable)
		return
	}

	if s.auth != nil {
		if _, _, err := s.sessionFromRequest(r); err != nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		if !auth.AllowedWebSocketOrigin(r) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Origin is validated via AllowedWebSocketOrigin when auth is enabled.
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Warn("websocket accept failed", "err", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := r.Context()
	rev, torrents, engine := s.engine.Snapshot()
	eng := engine
	snapshot := sync.Event{
		Type:     sync.EventSnapshot,
		Revision: rev,
		Torrents: torrents,
		Engine:   &eng,
	}
	if err := writeWSJSON(ctx, conn, snapshot); err != nil {
		return
	}

	ch := s.engine.Subscribe()
	defer s.engine.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if err := writeWSJSON(ctx, conn, ev); err != nil {
				return
			}
		}
	}
}

func writeWSJSON(ctx context.Context, conn *websocket.Conn, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func (s *Server) handleNoUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Fleet is running. Build the web UI and embed it, or open /api/v1/health.\n"))
}

func spaHandler(root fs.FS, files http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(root, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(payload)
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Debug("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}
