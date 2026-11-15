// Command echo-worker is a minimal Fleet extension worker for protocol tests.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      any             `json:"id"`
}

func main() {
	crash := flag.Bool("crash-after-handshake", false, "exit after successful handshake")
	flag.Parse()

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 8<<20)
	enc := json.NewEncoder(os.Stdout)

	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		switch req.Method {
		case "fleet.handshake":
			if *crash {
				os.Exit(2)
			}
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"ok": true, "protocolVersion": 1},
			})
		case "fleet.ping", "ext.activate", "ext.deactivate", "ext.upgrade":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"ok": true},
			})
		case "ext.health":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"ok": true},
			})
		case "ext.debug.exit":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"ok": true},
			})
			os.Exit(2)
		case "fleet.shutdown":
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"ok": true},
			})
			return
		default:
			_ = enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error": map[string]any{
					"code":    -32601,
					"message": fmt.Sprintf("method not found: %s", req.Method),
				},
			})
		}
	}
}
