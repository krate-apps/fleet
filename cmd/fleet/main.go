package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/krate-apps/fleet/internal/api"
	"github.com/krate-apps/fleet/internal/auth"
	"github.com/krate-apps/fleet/internal/buckets"
	"github.com/krate-apps/fleet/internal/config"
	"github.com/krate-apps/fleet/internal/extensions"
	"github.com/krate-apps/fleet/internal/fsops"
	"github.com/krate-apps/fleet/internal/gateways"
	"github.com/krate-apps/fleet/internal/geoip"
	"github.com/krate-apps/fleet/internal/intake"
	"github.com/krate-apps/fleet/internal/jobs"
	"github.com/krate-apps/fleet/internal/logs"
	"github.com/krate-apps/fleet/internal/rtorrent"
	"github.com/krate-apps/fleet/internal/security"
	"github.com/krate-apps/fleet/internal/share"
	"github.com/krate-apps/fleet/internal/storage"
	"github.com/krate-apps/fleet/internal/sync"
	"github.com/krate-apps/fleet/internal/themes"
	"github.com/krate-apps/fleet/internal/tools"
	"github.com/krate-apps/fleet/internal/updates"
	"github.com/krate-apps/fleet/internal/version"
)

//go:embed all:web
var embeddedWeb embed.FS

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "", "path to config.yaml")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("fleet %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuiltAt)
		return 0
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	logLevel := parseLevel(cfg.LogLevel)
	logRing := logs.NewRing(2000)
	logger := slog.New(logs.NewMultiHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}),
		logRing.Handler(logLevel),
	))

	if err := security.EnsurePrivateDir(cfg.DataDir); err != nil {
		logger.Error("data dir", "err", err)
		return 1
	}
	if err := cfg.EnsureDataDir(); err != nil {
		logger.Error("data dir", "err", err)
		return 1
	}
	if cfg.RTorrent.Address != "" {
		if err := security.CheckSocketAccess(cfg.RTorrent.Address); err != nil {
			logger.Warn("rtorrent socket check", "err", err)
		}
	}
	for _, root := range cfg.RTorrent.DownloadRoots {
		if err := security.CheckPathPermissions(root); err != nil {
			logger.Error("download_roots", "err", err)
			return 1
		}
	}

	store, err := storage.Open(cfg.DataDir)
	if err != nil {
		logger.Error("storage", "err", err)
		return 1
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("storage close", "err", err)
		}
	}()

	webFS, err := loadWebFS()
	if err != nil {
		logger.Warn("web assets unavailable", "err", err)
	}

	var rtClient *rtorrent.Client
	if cfg.RTorrent.Address != "" {
		rtClient, err = rtorrent.NewClient(cfg.RTorrent.Address)
		if err != nil {
			logger.Error("rtorrent address", "err", err)
			return 1
		}
		logger.Info("rtorrent configured", "address", rtClient.Endpoint().String())
	} else {
		logger.Warn("rtorrent address empty; engine sync disabled until configured")
	}

	engine := sync.NewEngine(rtClient, logger, sync.Options{})
	engine.SetGatewaySource(gatewaySyncSource{store: store})
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go engine.Run(ctx)

	jobMgr := jobs.NewManager(store)
	bucketReg, err := buckets.NewRegistry(store, cfg.DataDir)
	if err != nil {
		logger.Error("buckets", "err", err)
		return 1
	}
	officialPath := buckets.ResolveOfficialBucketPath(
		cfg.Buckets.OfficialPath,
		filepath.Join("..", "buckets", "fleet-official-bucket"),
		"packaging/official-bucket",
		filepath.Join("packaging", "official-bucket"),
	)
	if !cfg.Buckets.DisableOfficial && officialPath != "" {
		_ = buckets.SyncOfficialLocalPackages(officialPath, "extensions", cfg.ExtensionsDir)
		if rec, err := bucketReg.EnsureOfficialBucket(buckets.EnsureOfficialOptions{
			Path:     officialPath,
			Disabled: false,
		}); err != nil {
			logger.Warn("official bucket", "err", err)
		} else if rec != nil {
			logger.Info("official bucket ready", "id", rec.ID, "enabled", rec.Enabled, "path", rec.Path)
		}
	} else if cfg.Buckets.DisableOfficial {
		logger.Info("official bucket disabled by config")
	}
	extRoots := extensionRoots(cfg, bucketReg.InstallRoot())
	toolsRunner := &tools.Runner{
		MktorrentPath:   cfg.Tools.Mktorrent,
		DumptorrentPath: cfg.Tools.Dumptorrent,
		MediainfoPath:   cfg.Tools.Mediainfo,
		FfmpegPath:      cfg.Tools.Ffmpeg,
		SoxPath:         cfg.Tools.Sox,
		AllowedRoots:    append([]string(nil), cfg.RTorrent.DownloadRoots...),
		ArtifactsDir:    filepath.Join(cfg.DataDir, "artifacts"),
	}
	extBridge := &extensions.CoreBridge{
		Storage: &extensions.StorageBridge{DataDir: cfg.DataDir},
		Torrents: torrentsAdapter{
			engine:        engine,
			store:         store,
			roots:         append([]string(nil), cfg.RTorrent.DownloadRoots...),
			unrarBin:      cfg.Tools.Unrar,
			mediainfoPath: cfg.Tools.Mediainfo,
			ffmpegPath:    cfg.Tools.Ffmpeg,
			soxPath:       cfg.Tools.Sox,
		},
		Gateways: gatewaysAdapter{store: store},
		Intake:   intakeAdapter{store: store, engine: engine},
		Jobs:     jobMgr,
		Throttle: throttleAdapter{engine},
		Engine:   engineAdapter{engine: engine},
		Files: extensions.FilesAdapter{M: &fsops.Manager{
			AllowedRoots: append([]string(nil), cfg.RTorrent.DownloadRoots...),
			HideDot:      true,
		}},
		URLRewrite: &extensions.URLRewriteStore{DataDir: cfg.DataDir},
		Cookies:    &extensions.CookieStore{DataDir: cfg.DataDir},
		Tools:      toolsRunner,
		DataDir:    cfg.DataDir,
	}
	extBridge.LoginMgr = &extensions.LoginStore{
		DataDir: cfg.DataDir,
		Cookies: extBridge.Cookies,
		HTTP:    extBridge.HTTP,
	}
	extHost := extensions.NewHostWithOptions(ctx, store, extensions.HostOptions{
		Roots:   extRoots,
		DataDir: cfg.DataDir,
		Bridge:  extBridge,
		WorkerEnv: map[string]string{
			"FLEET_TOOL_RCLONE": cfg.Tools.Rclone,
		},
	})
	if err := extHost.Refresh(); err != nil {
		logger.Warn("extensions refresh", "err", err)
	} else {
		logger.Info("extensions discovered", "count", len(extHost.List()), "roots", extRoots)
	}

	var authSvc *auth.Service
	if !cfg.Auth.Disable {
		authSvc, err = auth.NewService(store, cfg.Auth.SecureCookies)
		if err != nil {
			logger.Error("auth", "err", err)
			return 1
		}
		logger.Info("auth enabled")
	} else {
		logger.Warn("auth disabled by config; API is open on this instance")
	}
	geoDB, err := geoip.Open(cfg.GeoIP.Database)
	if err != nil {
		logger.Error("geoip", "err", err)
		return 1
	}
	if geoDB != nil {
		defer func() {
			if cerr := geoDB.Close(); cerr != nil {
				logger.Error("geoip close", "err", cerr)
			}
		}()
		logger.Info("geoip enabled", "database", geoDB.Path())
	}

	updateMgr := updates.NewManager(store, cfg.DataDir, updates.ManagerConfig{
		InstallPrefix: cfg.InstallPrefix,
		GitHubRepo:    cfg.Updates.GitHubRepo,
		AssetPattern:  cfg.Updates.AssetPattern,
	})
	logAgg := logs.NewAggregator(logRing, logs.Config{
		InstallPrefix:   cfg.InstallPrefix,
		RTorrentAddress: cfg.RTorrent.Address,
	})
	themeStore := &themes.Store{Root: filepath.Join(cfg.DataDir, "themes-src")}
	srv := api.New(cfg, logger, webFS, engine, api.Options{
		Jobs:       jobMgr,
		Extensions: extHost,
		Buckets:    bucketReg,
		Auth:       authSvc,
		Updates:    updateMgr,
		GeoIP:      geoDB,
		Store:      store,
		Shares:     &share.Store{DataDir: cfg.DataDir},
		Logs:       logAgg,
		Themes:     themeStore,
	})
	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("fleet listening",
			"addr", cfg.Addr(),
			"version", version.Version,
			"data_dir", cfg.DataDir,
		)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
			return 1
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
		return 1
	}
	return 0
}

type throttleAdapter struct {
	engine *sync.Engine
}

func (a throttleAdapter) GetGlobalRates(ctx context.Context) (int64, int64, error) {
	r, err := a.engine.GetGlobalRates(ctx)
	if err != nil {
		return 0, 0, err
	}
	return r.DownRate, r.UpRate, nil
}

func (a throttleAdapter) SetGlobalRates(ctx context.Context, downRate, upRate int64) error {
	return a.engine.SetGlobalRates(ctx, downRate, upRate)
}

func (a throttleAdapter) EnsureNamed(ctx context.Context, name string, upRate, downRate int64) error {
	return a.engine.EnsureNamedThrottle(ctx, name, upRate, downRate)
}

type engineAdapter struct {
	engine *sync.Engine
}

func (a engineAdapter) Capabilities() map[string]any {
	if a.engine == nil {
		return map[string]any{"connected": false}
	}
	return a.engine.Capabilities()
}

type torrentsAdapter struct {
	engine        *sync.Engine
	store         *storage.Store
	roots         []string
	unrarBin      string
	mediainfoPath string
	ffmpegPath    string
	soxPath       string
}

func (a torrentsAdapter) AddMagnets(ctx context.Context, magnets []string, start bool) error {
	_, err := intake.Add(ctx, a.store, a.engine, intake.Request{Magnets: magnets, Start: start})
	return err
}

func (a torrentsAdapter) AddTorrentFile(ctx context.Context, data []byte, start bool) error {
	_, err := intake.Add(ctx, a.store, a.engine, intake.Request{Torrent: data, Start: start})
	return err
}

func (a torrentsAdapter) TorrentMetainfo(ctx context.Context, hash string) (string, string, error) {
	return a.engine.TorrentMetainfo(ctx, hash, a.roots)
}

func (a torrentsAdapter) ResolveSource(ctx context.Context, hash string) (string, []byte, error) {
	path, filename, err := a.engine.ResolveSourceTorrent(ctx, hash, a.roots)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return filename, data, nil
}

type gatewaySyncSource struct {
	store *storage.Store
}

func (s gatewaySyncSource) ListGateways(_ context.Context) ([]sync.GatewayEndpoint, error) {
	if s.store == nil {
		return nil, nil
	}
	list, err := s.store.ListGatewayClientsWithSecrets()
	if err != nil {
		return nil, err
	}
	out := make([]sync.GatewayEndpoint, 0, len(list))
	for _, c := range list {
		out = append(out, sync.GatewayEndpoint{
			ID:       c.ID,
			Kind:     c.Kind,
			Name:     c.Name,
			URL:      c.URL,
			Username: c.Username,
			Password: c.Password,
			Enabled:  c.Enabled,
		})
	}
	return out, nil
}

type intakeAdapter struct {
	store  *storage.Store
	engine *sync.Engine
}

func (a intakeAdapter) Add(ctx context.Context, magnets []string, torrent []byte, filename string, start bool, gatewayID string) (string, error) {
	return intake.Add(ctx, a.store, a.engine, intake.Request{
		Magnets:   magnets,
		Torrent:   torrent,
		Filename:  filename,
		Start:     start,
		GatewayID: gatewayID,
	})
}

type gatewaysAdapter struct {
	store *storage.Store
}

func (a gatewaysAdapter) ListGateways(_ context.Context) ([]extensions.ExtGateway, error) {
	if a.store == nil {
		return []extensions.ExtGateway{}, nil
	}
	list, err := a.store.ListGatewayClients()
	if err != nil {
		return nil, err
	}
	out := make([]extensions.ExtGateway, 0, len(list))
	for _, c := range list {
		full, err := a.store.GetGatewayClient(c.ID)
		if err != nil {
			continue
		}
		out = append(out, extensions.ExtGateway{
			ID:       full.ID,
			Kind:     string(full.Kind),
			Name:     full.Name,
			URL:      full.URL,
			Username: full.Username,
			Password: full.Password,
			Enabled:  full.Enabled,
		})
	}
	return out, nil
}

func (a gatewaysAdapter) AddTorrent(ctx context.Context, gatewayID, filename string, torrent []byte, savePath string, skipIfPresent bool) (bool, bool, string, error) {
	if a.store == nil {
		return false, false, "", fmt.Errorf("storage unavailable")
	}
	c, err := a.store.GetGatewayClient(gatewayID)
	if err != nil {
		return false, false, "", err
	}
	if !c.Enabled {
		return false, false, "", fmt.Errorf("gateway disabled")
	}
	res := gateways.AddTorrent(ctx, gateways.AddInput{
		Kind:          c.Kind,
		URL:           c.URL,
		Username:      c.Username,
		Password:      c.Password,
		Filename:      filename,
		Torrent:       torrent,
		SavePath:      savePath,
		SkipIfPresent: skipIfPresent,
	})
	if !res.OK {
		return false, res.Skipped, res.Message, fmt.Errorf("%s", res.Message)
	}
	return true, res.Skipped, res.Message, nil
}

func (a gatewaysAdapter) SetTorrentLabel(ctx context.Context, gatewayID, hash, label string) error {
	if a.store == nil {
		return fmt.Errorf("storage unavailable")
	}
	c, err := a.store.GetGatewayClient(gatewayID)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return fmt.Errorf("gateway disabled")
	}
	res := gateways.SetTorrentLabel(ctx, gateways.LabelInput{
		Kind:     c.Kind,
		URL:      c.URL,
		Username: c.Username,
		Password: c.Password,
		Hash:     hash,
		Label:    label,
	})
	if !res.OK {
		return fmt.Errorf("%s", res.Message)
	}
	return nil
}

func (a torrentsAdapter) ListExtTorrents(ctx context.Context, allClients bool) ([]extensions.ExtTorrent, error) {
	rows, err := a.engine.ListExtTorrents(ctx, allClients)
	if err != nil {
		return nil, err
	}
	out := make([]extensions.ExtTorrent, 0, len(rows))
	for _, r := range rows {
		out = append(out, extensions.ExtTorrent{
			Hash:         r.Hash,
			Name:         r.Name,
			Ratio:        r.Ratio,
			Label:        r.Label,
			State:        r.State,
			SizeBytes:    r.SizeBytes,
			DownTotal:    r.DownTotal,
			UpTotal:      r.UpTotal,
			DownRate:     r.DownRate,
			UpRate:       r.UpRate,
			Complete:     r.Complete,
			FinishedAt:   r.FinishedAt,
			Directory:    r.Directory,
			ThrottleName: r.ThrottleName,
			Private:      r.Private,
			ClientID:     r.ClientID,
		})
	}
	return out, nil
}

func (a torrentsAdapter) ListTrackers(ctx context.Context, hash string) ([]extensions.ExtTracker, error) {
	rows, err := a.engine.ListTrackers(ctx, hash)
	if err != nil {
		return nil, err
	}
	out := make([]extensions.ExtTracker, 0, len(rows))
	for _, r := range rows {
		out = append(out, extensions.ExtTracker{
			Index:   r.Index,
			ID:      r.ID,
			URL:     r.URL,
			Enabled: r.Enabled,
		})
	}
	return out, nil
}

func (a torrentsAdapter) AddTracker(ctx context.Context, hash, trackerURL string) error {
	return a.engine.AddTracker(ctx, hash, trackerURL)
}

func (a torrentsAdapter) ListPeers(ctx context.Context, hash string) ([]extensions.ExtPeer, error) {
	rows, err := a.engine.ListPeers(ctx, hash)
	if err != nil {
		return nil, err
	}
	out := make([]extensions.ExtPeer, 0, len(rows))
	for _, r := range rows {
		out = append(out, extensions.ExtPeer{
			ID:            r.ID,
			Address:       r.Address,
			ClientVersion: r.ClientVersion,
		})
	}
	return out, nil
}

func (a torrentsAdapter) DisconnectPeer(ctx context.Context, hash, address string) error {
	return a.engine.DisconnectPeer(ctx, hash, address)
}

func (a torrentsAdapter) SetDirectory(ctx context.Context, hash, directory string) error {
	return a.engine.SetDirectory(ctx, hash, directory)
}

func (a torrentsAdapter) AssignThrottles(ctx context.Context, hashes []string, throttleName string) error {
	return a.engine.AssignTorrentThrottles(ctx, hashes, throttleName)
}

func (a torrentsAdapter) ApplyExtAction(ctx context.Context, action string, hashes []string) error {
	return a.engine.ApplyExtAction(ctx, action, hashes)
}

func (a torrentsAdapter) SetLabel(ctx context.Context, hash, label string) error {
	return a.engine.SetLabel(ctx, hash, label)
}

func (a torrentsAdapter) SetRates(ctx context.Context, hash, clientID string, upBps, downBps int64) error {
	return a.engine.SetRatesOnClient(ctx, hash, clientID, upBps, downBps)
}

func (a torrentsAdapter) UnpackArchive(ctx context.Context, archivePath, destDir string) (int, error) {
	return a.engine.UnpackArchive(ctx, archivePath, destDir, sync.UnpackOptions{
		DownloadRoots: a.roots,
		UnrarBin:      a.unrarBin,
	})
}

func (a torrentsAdapter) MediaInfo(ctx context.Context, hash string, index int, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		resolved, _, err := a.engine.ResolveDataFile(ctx, hash, index, a.roots)
		if err != nil {
			return "", err
		}
		path = resolved
	}
	runner := &tools.Runner{
		MediainfoPath: a.mediainfoPath,
		AllowedRoots:  append([]string(nil), a.roots...),
	}
	return runner.MediaInfo(ctx, path)
}

func (a torrentsAdapter) Screenshots(ctx context.Context, hash string, index int, path string, count, offset, interval, width int) ([]map[string]any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		resolved, _, err := a.engine.ResolveDataFile(ctx, hash, index, a.roots)
		if err != nil {
			return nil, err
		}
		path = resolved
	}
	runner := &tools.Runner{
		FfmpegPath:   a.ffmpegPath,
		AllowedRoots: append([]string(nil), a.roots...),
	}
	frames, err := runner.Screenshots(ctx, path, tools.ScreenshotOpts{
		Count: count, Offset: offset, Interval: interval, Width: width,
	})
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(frames))
	for _, f := range frames {
		out = append(out, map[string]any{
			"index":     f.Index,
			"offsetSec": f.Offset,
			"mime":      f.Mime,
			"dataUrl":   f.DataURL,
			"bytes":     f.Bytes,
		})
	}
	return out, nil
}

func (a torrentsAdapter) Spectrogram(ctx context.Context, hash string, index int, path, title string, width int) (map[string]any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		resolved, _, err := a.engine.ResolveDataFile(ctx, hash, index, a.roots)
		if err != nil {
			return nil, err
		}
		path = resolved
	}
	runner := &tools.Runner{
		SoxPath:      a.soxPath,
		AllowedRoots: append([]string(nil), a.roots...),
	}
	img, err := runner.Spectrogram(ctx, path, tools.SpectrogramOpts{Title: title, Width: width})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"mime":    img.Mime,
		"dataUrl": img.DataURL,
		"bytes":   img.Bytes,
	}, nil
}

func extensionRoots(cfg config.Config, installRoot string) []string {
	roots := make([]string, 0, 4)
	if cfg.ExtensionsDir != "" {
		roots = append(roots, cfg.ExtensionsDir)
	}
	if installRoot != "" {
		roots = append(roots, installRoot)
	} else {
		roots = append(roots, filepath.Join(cfg.DataDir, "extensions-src"))
	}
	// Bundled official reference packages when running from a source checkout.
	if st, err := os.Stat("extensions"); err == nil && st.IsDir() {
		roots = append(roots, "extensions")
	}
	return roots
}

func loadWebFS() (fs.FS, error) {
	sub, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return nil, err
	}
	// Placeholder directory only contains .gitkeep until the UI is built into cmd/fleet/web.
	for _, e := range entries {
		if e.Name() != ".gitkeep" {
			return sub, nil
		}
	}
	return nil, fmt.Errorf("no built web assets under cmd/fleet/web")
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
