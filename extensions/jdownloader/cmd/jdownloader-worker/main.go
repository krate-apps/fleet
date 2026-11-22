// Command jdownloader-worker connects to MyJDownloader for queue status and controls.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      any             `json:"id"`
}

type settings struct {
	Enabled    bool   `json:"enabled"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"deviceName"`
}

type hostReply struct {
	result json.RawMessage
	err    string
}

var (
	mu       sync.Mutex
	nextHost = int64(1)
	pending  = map[int64]chan hostReply{}
	outMu    sync.Mutex

	clientMu sync.Mutex
	jdClient *myjdClient
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for in.Scan() {
		line := append([]byte(nil), in.Bytes()...)
		if len(line) == 0 {
			continue
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(line, &probe); err != nil {
			continue
		}
		if _, hasMethod := probe["method"]; !hasMethod {
			var resp struct {
				ID     any             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}
			idNum, ok := asInt64(resp.ID)
			if !ok {
				continue
			}
			mu.Lock()
			ch := pending[idNum]
			delete(pending, idNum)
			mu.Unlock()
			if ch == nil {
				continue
			}
			msg := ""
			if resp.Error != nil {
				msg = resp.Error.Message
			}
			ch <- hostReply{result: resp.Result, err: msg}
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		go handleRequest(req)
	}
}

func handleRequest(req request) {
	switch req.Method {
	case "fleet.handshake", "fleet.ping", "ext.activate", "ext.deactivate", "ext.health", "ext.upgrade":
		reply(req.ID, map[string]any{"ok": true})
	case "fleet.shutdown":
		reply(req.ID, map[string]any{"ok": true})
		os.Exit(0)
	case "ext.jdownloader.getSettings":
		st, err := loadSettings()
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, publicSettings(st))
	case "ext.jdownloader.saveSettings":
		var p settings
		if err := json.Unmarshal(req.Params, &p); err != nil {
			replyErr(req.ID, "invalid settings")
			return
		}
		cur, _ := loadSettings()
		var extra struct {
			ClearPassword bool `json:"clearPassword"`
		}
		_ = json.Unmarshal(req.Params, &extra)
		if strings.TrimSpace(p.Password) == "" && !extra.ClearPassword {
			p.Password = cur.Password
		}
		if extra.ClearPassword {
			p.Password = ""
		}
		p.Email = strings.TrimSpace(p.Email)
		p.DeviceName = strings.TrimSpace(p.DeviceName)
		if err := saveSettings(&p); err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		invalidateClient()
		reply(req.ID, publicSettings(p))
	case "ext.jdownloader.testConnection":
		res, err := testConnection()
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, res)
	case "ext.jdownloader.listQueue":
		res, err := listQueue()
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, res)
	case "ext.jdownloader.control":
		var p struct {
			Action string `json:"action"`
			ID     any    `json:"id"`
			IDs    any    `json:"ids"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			replyErr(req.ID, "invalid control params")
			return
		}
		res, err := control(p.Action, p.ID, p.IDs)
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, res)
	default:
		replyErr(req.ID, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func defaultSettings() settings {
	return settings{Enabled: false}
}

func publicSettings(st settings) map[string]any {
	return map[string]any{
		"enabled":            st.Enabled,
		"email":              st.Email,
		"deviceName":         st.DeviceName,
		"passwordConfigured": strings.TrimSpace(st.Password) != "",
	}
}

func loadSettings() (settings, error) {
	raw, err := hostCall("fleet.storage.get", map[string]any{"key": "settings"})
	if err != nil {
		return settings{}, err
	}
	if string(raw) == "null" || len(raw) == 0 {
		return defaultSettings(), nil
	}
	var st settings
	if err := json.Unmarshal(raw, &st); err != nil {
		return defaultSettings(), nil
	}
	return st, nil
}

func saveSettings(st *settings) error {
	_, err := hostCall("fleet.storage.put", map[string]any{"key": "settings", "value": st})
	return err
}

func invalidateClient() {
	clientMu.Lock()
	jdClient = nil
	clientMu.Unlock()
}

func getClient(st settings) (*myjdClient, error) {
	if strings.TrimSpace(st.Email) == "" || strings.TrimSpace(st.Password) == "" {
		return nil, fmt.Errorf("configure MyJD email and password")
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if jdClient == nil || jdClient.email != st.Email || jdClient.password != st.Password {
		jdClient = newMyJD(st.Email, st.Password)
	}
	return jdClient, nil
}

func testConnection() (map[string]any, error) {
	st, err := loadSettings()
	if err != nil {
		return nil, err
	}
	cli, err := getClient(st)
	if err != nil {
		return nil, err
	}
	if err := cli.connect(); err != nil {
		return nil, err
	}
	deviceID, err := cli.resolveDeviceID(st.DeviceName)
	if err != nil {
		return nil, err
	}
	cli.mu.Lock()
	names := make([]string, 0, len(cli.devices))
	for _, d := range cli.devices {
		names = append(names, d.Name)
	}
	cli.mu.Unlock()
	return map[string]any{
		"ok":       true,
		"deviceId": deviceID,
		"devices":  names,
	}, nil
}

func listQueue() (map[string]any, error) {
	st, err := loadSettings()
	if err != nil {
		return nil, err
	}
	cli, err := getClient(st)
	if err != nil {
		return nil, err
	}
	deviceID, err := cli.resolveDeviceID(st.DeviceName)
	if err != nil {
		return nil, err
	}
	query := map[string]any{
		"bytesLoaded": true,
		"bytesTotal":  true,
		"comment":     true,
		"enabled":     true,
		"eta":         true,
		"finished":    true,
		"maxResults":  -1,
		"running":     true,
		"saveTo":      true,
		"speed":       true,
		"startAt":     0,
		"status":      true,
	}
	raw, err := cli.deviceAction(deviceID, "/downloadsV2/queryPackages", []any{query})
	if err != nil {
		return nil, err
	}
	items, err := normalizeMyJDPackages(raw)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "source": "myjd", "at": time.Now().UTC().Format(time.RFC3339)}, nil
}

func control(action string, id any, ids any) (map[string]any, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	st, err := loadSettings()
	if err != nil {
		return nil, err
	}
	cli, err := getClient(st)
	if err != nil {
		return nil, err
	}
	deviceID, err := cli.resolveDeviceID(st.DeviceName)
	if err != nil {
		return nil, err
	}
	pkgIDs := parseIDList(ids)
	if len(pkgIDs) == 0 {
		pkgIDs = parseIDList(id)
	}
	emptyLinks := []any{}

	switch action {
	case "pause", "stop":
		_, err = cli.deviceAction(deviceID, "/downloadcontroller/pause", []any{true})
		if err != nil {
			return nil, err
		}
		if len(pkgIDs) > 0 {
			_, err = cli.deviceAction(deviceID, "/downloadsV2/setEnabled", []any{false, emptyLinks, pkgIDs})
		}
	case "resume", "start", "unpause":
		if len(pkgIDs) > 0 {
			_, err = cli.deviceAction(deviceID, "/downloadsV2/setEnabled", []any{true, emptyLinks, pkgIDs})
			if err != nil {
				return nil, err
			}
		}
		_, err = cli.deviceAction(deviceID, "/downloadcontroller/start", nil)
	case "remove", "delete":
		if len(pkgIDs) == 0 {
			return nil, fmt.Errorf("id required for remove")
		}
		_, err = cli.deviceAction(deviceID, "/downloadsV2/removeLinks", []any{emptyLinks, pkgIDs})
	case "force":
		if len(pkgIDs) == 0 {
			return nil, fmt.Errorf("id required for force")
		}
		_, err = cli.deviceAction(deviceID, "/downloadsV2/forceDownload", []any{emptyLinks, pkgIDs})
	default:
		return nil, fmt.Errorf("unknown action %q (pause|resume|stop|remove|force)", action)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "action": action}, nil
}

func hostCall(method string, params any) (json.RawMessage, error) {
	ch := make(chan hostReply, 1)
	mu.Lock()
	id := nextHost
	nextHost++
	pending[id] = ch
	mu.Unlock()

	b, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      id,
	})
	if err != nil {
		mu.Lock()
		delete(pending, id)
		mu.Unlock()
		return nil, err
	}
	outMu.Lock()
	fmt.Fprintf(os.Stdout, "%s\n", b)
	outMu.Unlock()

	select {
	case reply := <-ch:
		if reply.err != "" {
			return nil, fmt.Errorf("%s", reply.err)
		}
		if len(reply.result) == 0 {
			return json.RawMessage("null"), nil
		}
		return reply.result, nil
	case <-time.After(30 * time.Second):
		mu.Lock()
		delete(pending, id)
		mu.Unlock()
		return nil, fmt.Errorf("host call timeout")
	}
}

func reply(id any, result any) {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "result": result, "id": id})
	outMu.Lock()
	fmt.Fprintf(os.Stdout, "%s\n", b)
	outMu.Unlock()
}

func replyErr(id any, msg string) {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"error":   map[string]any{"code": -32000, "message": msg},
		"id":      id,
	})
	outMu.Lock()
	fmt.Fprintf(os.Stdout, "%s\n", b)
	outMu.Unlock()
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}
