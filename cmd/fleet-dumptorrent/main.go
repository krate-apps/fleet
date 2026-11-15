// Fleet reference dumptorrent JSON CLI (OPEN-11).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/krate-apps/fleet/internal/metainfo"
)

func main() {
	args := os.Args[1:]
	jsonMode := false
	var files []string
	for _, a := range args {
		switch a {
		case "--json", "-j", "-J":
			jsonMode = true
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, "usage: fleet-dumptorrent [--json|-j|-J] <file.torrent>\n")
			os.Exit(0)
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(os.Stderr, "unknown option %q\n", a)
				os.Exit(2)
			}
			files = append(files, a)
		}
	}
	if len(files) != 1 {
		fmt.Fprintf(os.Stderr, "usage: fleet-dumptorrent [--json|-j|-J] <file.torrent>\n")
		os.Exit(2)
	}
	sum, err := metainfo.ParseTorrentFile(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "dumptorrent: %v\n", err)
		os.Exit(1)
	}
	if !jsonMode {
		fmt.Fprintf(os.Stderr, "JSON flag required (--json|-j|-J) for Fleet OPEN-11\n")
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(sum); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}
