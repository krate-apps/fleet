// Command rtprobe prints a JSON OPEN-02 matrix row for a live rTorrent SCGI endpoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

func main() {
	addr := flag.String("address", "", "SCGI address (unix:///path.sock or scgi://host:port)")
	timeout := flag.Duration("timeout", 8*time.Second, "RPC timeout")
	flag.Parse()
	if *addr == "" {
		fmt.Fprintln(os.Stderr, "usage: rtprobe -address unix:///path/to/rtorrent.sock")
		os.Exit(2)
	}
	client, err := rtorrent.NewClient(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "address: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	row := client.ProbeMatrix(ctx)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(row); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
	if !row.Connected {
		os.Exit(1)
	}
}
