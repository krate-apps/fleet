// Command pyload-worker connects to a local pyLoad JSON API for queue status and controls.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
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
	Enabled  bool   `json:"enabled"`
	BaseURL  string `json:"baseUrl"`
	Username string `json:"username"`
	Password string `json:"password"`
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
	pyCli    *pyClient
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
	case "ext.pyload.getSettings":
		st, err := loadSettings()
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, publicSettings(st))
	case "ext.pyload.saveSettings":
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
		p.BaseURL = strings.TrimSpace(p.BaseURL)
		p.Username = strings.TrimSpace(p.Username)
		if p.BaseURL != "" {
			if _, err := validatePrivateBaseURL(p.BaseURL); err != nil {
				replyErr(req.ID, err.Error())
				return
			}
		}
		if err := saveSettings(&p); err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		invalidateClient()
		reply(req.ID, publicSettings(p))
	case "ext.pyload.testConnection":
		res, err := testConnection()
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, res)
	case "ext.pyload.listQueue":
		res, err := listQueue()
		if err != nil {
			replyErr(req.ID, err.Error())
			return
		}
		reply(req.ID, res)
	case "ext.pyload.control":
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
	return settings{Enabled: false, BaseURL: "http://127.0.0.1:8000"}
}

func publicSettings(st settings) map[string]any {
	return map[string]any{
		"enabled":            st.Enabled,
		"baseUrl":            st.BaseURL,
		"username":           st.Username,
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
	if strings.TrimSpace(st.BaseURL) == "" {
		st.BaseURL = defaultSettings().BaseURL
	}
	return st, nil
}

func saveSettings(st *settings) error {
	_, err := hostCall("fleet.storage.put", map[string]any{"key": "settings", "value": st})
	return err
}

func invalidateClient() {
	clientMu.Lock()
	pyCli = nil
	clientMu.Unlock()
}

func getClient(st settings) (*pyClient, error) {
	if strings.TrimSpace(st.BaseURL) == "" {
		return nil, fmt.Errorf("configure pyLoad base URL")
	}
	pass := strings.TrimSpace(st.Password)
	if pass == "" {
		return nil, fmt.Errorf("configure pyLoad password or pl_ API key")
	}
	if !strings.HasPrefix(pass, "pl_") && strings.TrimSpace(st.Username) == "" {
		return nil, fmt.Errorf("configure pyLoad username and password (or a pl_ API key)")
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	needNew := pyCli == nil || pyCli.username != st.Username || pyCli.password != st.Password
	if !needNew && pyCli != nil {
		cur := strings.TrimRight(pyCli.base.String(), "/")
		want := strings.TrimRight(st.BaseURL, "/")
		needNew = cur != want
	}
	if needNew {
		cli, err := newPyClient(st.BaseURL, st.Username, st.Password)
		if err != nil {
			return nil, err
		}
		pyCli = cli
	}
	return pyCli, nil
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
	if err := cli.login(); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
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
	raw, err := cli.getJSON("/api/statusDownloads")
	if err != nil {
		// Retry once after re-login.
		_ = cli.login()
		raw, err = cli.getJSON("/api/statusDownloads")
		if err != nil {
			return nil, err
		}
	}
	items, err := normalizePyStatus(raw)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "source": "pyload", "at": time.Now().UTC().Format(time.RFC3339)}, nil
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
	idList := collectIDs(id, ids)

	switch action {
	case "pause":
		_, err = cli.getJSON("/api/pause")
	case "resume", "unpause", "start":
		_, err = cli.getJSON("/api/unpause")
	case "stop":
		_, err = cli.getJSON("/api/stopAllDownloads")
	case "abort":
		if len(idList) == 0 {
			return nil, fmt.Errorf("id required for abort")
		}
		form := url.Values{}
		form.Set("fids", jsonArray(idList))
		_, err = cli.callForm("/api/stopDownloads", form)
	case "delete", "remove":
		if len(idList) == 0 {
			return nil, fmt.Errorf("id required for delete")
		}
		form := url.Values{}
		form.Set("fids", jsonArray(idList))
		_, err = cli.callForm("/api/deleteFiles", form)
	case "restart":
		if len(idList) == 0 {
			return nil, fmt.Errorf("id required for restart")
		}
		form := url.Values{}
		form.Set("fid", idList[0])
		_, err = cli.callForm("/api/restartFile", form)
	default:
		return nil, fmt.Errorf("unknown action %q (pause|resume|stop|abort|delete|restart)", action)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "action": action}, nil
}

func collectIDs(id any, ids any) []string {
	out := []string{}
	add := func(v any) {
		switch t := v.(type) {
		case nil:
		case []any:
			for _, x := range t {
				if s := strings.TrimSpace(fmt.Sprint(x)); s != "" {
					out = append(out, s)
				}
			}
		default:
			if s := strings.TrimSpace(fmt.Sprint(t)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
	}
	add(ids)
	if len(out) == 0 {
		add(id)
	}
	return out
}

func jsonArray(ids []string) string {
	b, _ := json.Marshal(ids)
	return string(b)
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
