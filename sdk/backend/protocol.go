// Package backend defines JSON-RPC 2.0 framing helpers for extension workers.
package backend

import (
	"encoding/json"
	"fmt"
)

const ProtocolVersion = 1

// Method names used between Fleet and workers.
const (
	MethodHandshake  = "fleet.handshake"
	MethodPing       = "fleet.ping"
	MethodShutdown   = "fleet.shutdown"
	MethodActivate   = "ext.activate"
	MethodDeactivate = "ext.deactivate"
	MethodHealth     = "ext.health"
	MethodUpgrade    = "ext.upgrade"
	NotifyEvent      = "fleet.event"
	NotifyLog        = "ext.log"
	NotifyJobProg    = "ext.job.progress"

	// Worker → Fleet reverse calls (capabilities gated by the host).
	MethodTorrentsAdd      = "fleet.torrents.add"
	MethodIntakeAdd        = "fleet.intake.add"
	MethodTorrentsList     = "fleet.torrents.list"
	MethodTorrentsAction   = "fleet.torrents.action"
	MethodTorrentsLabel      = "fleet.torrents.setLabel"
	MethodTorrentsSetRates   = "fleet.torrents.setRates"
	MethodTorrentsSetDirectory = "fleet.torrents.setDirectory"
	MethodTorrentsMetainfo = "fleet.torrents.metainfo"
	MethodTorrentsSource   = "fleet.torrents.source"
	MethodGatewaysList       = "fleet.gateways.list"
	MethodGatewaysAddTorrent = "fleet.gateways.addTorrent"
	MethodFilesUnpack      = "fleet.files.unpack"
	MethodFilesMediainfo   = "fleet.files.mediainfo"
	MethodFilesScreenshots = "fleet.files.screenshots"
	MethodFilesSpectrogram = "fleet.files.spectrogram"
	MethodFilesList        = "fleet.files.list"
	MethodFilesMkdir       = "fleet.files.mkdir"
	MethodFilesRename      = "fleet.files.rename"
	MethodFilesRemove      = "fleet.files.remove"
	MethodFilesCopy        = "fleet.files.copy"
	MethodFilesMove        = "fleet.files.move"
	MethodFilesReadText    = "fleet.files.readText"
	MethodFilesReadBase64  = "fleet.files.readBase64"
	MethodFilesChecksum    = "fleet.files.checksum"
	MethodFilesStat        = "fleet.files.stat"
	MethodFilesFingerprint = "fleet.files.fingerprint"
	MethodFilesHardlink    = "fleet.files.hardlink"
	MethodFilesRoots       = "fleet.files.roots"
	MethodProcessExec    = "fleet.process.exec"
	MethodHTTPFetch      = "fleet.http.fetch"
	MethodStorageGet     = "fleet.storage.get"
	MethodStoragePut     = "fleet.storage.put"
	MethodJobsCreate     = "fleet.jobs.create"
	MethodJobsUpdate     = "fleet.jobs.update"
	MethodJobsGet        = "fleet.jobs.get"
	MethodJobsList       = "fleet.jobs.list"
	MethodJobsCancel     = "fleet.jobs.cancel"
	MethodToolsMktorrent = "fleet.tools.mktorrent"
	MethodSystemStatus         = "fleet.system.status"
	MethodEngineCapabilities   = "fleet.engine.capabilities"
	MethodThrottleGet    = "fleet.throttle.get"
	MethodThrottleSet    = "fleet.throttle.set"
	MethodThrottleDefine = "fleet.throttle.define"
	MethodThrottleAssign = "fleet.throttle.assign"
	MethodTrackersList   = "fleet.trackers.list"
	MethodTrackersAdd    = "fleet.trackers.add"
	MethodPeersList       = "fleet.peers.list"
	MethodPeersDisconnect = "fleet.peers.disconnect"
	MethodURLRewriteList  = "fleet.urlrewrite.list"
	MethodURLRewriteSet   = "fleet.urlrewrite.set"
	MethodURLRewriteApply = "fleet.urlrewrite.apply"
	MethodCookiesList     = "fleet.cookies.list"
	MethodCookiesSet      = "fleet.cookies.set"
	MethodLoginMgrList    = "fleet.loginmgr.list"
	MethodLoginMgrSet     = "fleet.loginmgr.set"
	MethodLoginMgrLogin   = "fleet.loginmgr.login"
)

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
	ID      any             `json:"id"`
}

// ErrorObject is a JSON-RPC error payload.
type ErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (no id).
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// HandshakeParams is sent with fleet.handshake.
type HandshakeParams struct {
	ProtocolVersion  int      `json:"protocolVersion"`
	ExtensionID      string   `json:"extensionId"`
	ExtensionVersion string   `json:"extensionVersion"`
	Capabilities     []string `json:"capabilities,omitempty"`
}

// NewRequest builds a JSON-RPC request envelope.
func NewRequest(method string, id any, params any) (*Request, error) {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	return &Request{JSONRPC: "2.0", Method: method, Params: raw, ID: id}, nil
}

// DecodeHandshake validates handshake params.
func DecodeHandshake(params json.RawMessage) (*HandshakeParams, error) {
	var h HandshakeParams
	if err := json.Unmarshal(params, &h); err != nil {
		return nil, fmt.Errorf("handshake params: %w", err)
	}
	if h.ProtocolVersion != ProtocolVersion {
		return nil, fmt.Errorf("unsupported protocolVersion %d", h.ProtocolVersion)
	}
	if h.ExtensionID == "" || h.ExtensionVersion == "" {
		return nil, fmt.Errorf("extensionId and extensionVersion required")
	}
	return &h, nil
}
