// Fleet reference mktorrent JSON CLI (OPEN-11).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/krate-apps/fleet/internal/metainfo"
)

func main() {
	args := os.Args[1:]
	var (
		jsonMode bool
		private  bool
		outPath  string
		name     string
		comment  string
		pieceLog int
		announce []string
		paths    []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json" || a == "-j" || a == "-J":
			jsonMode = true
		case a == "-p" || a == "--private":
			private = true
		case a == "-h" || a == "--help":
			fmt.Fprintf(os.Stderr, "usage: fleet-mktorrent --json -a <url> [-o out] [-n name] [-c comment] [-l log2piece] [-p] <paths…>\n")
			os.Exit(0)
		case a == "-a" || strings.HasPrefix(a, "--announce="):
			val := ""
			if strings.HasPrefix(a, "--announce=") {
				val = strings.TrimPrefix(a, "--announce=")
			} else {
				i++
				if i >= len(args) {
					fatal("missing -a value")
				}
				val = args[i]
			}
			for _, part := range strings.Split(val, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					announce = append(announce, part)
				}
			}
		case a == "-o" || strings.HasPrefix(a, "--output="):
			if strings.HasPrefix(a, "--output=") {
				outPath = strings.TrimPrefix(a, "--output=")
			} else {
				i++
				if i >= len(args) {
					fatal("missing -o value")
				}
				outPath = args[i]
			}
		case a == "-n" || strings.HasPrefix(a, "--name="):
			if strings.HasPrefix(a, "--name=") {
				name = strings.TrimPrefix(a, "--name=")
			} else {
				i++
				if i >= len(args) {
					fatal("missing -n value")
				}
				name = args[i]
			}
		case a == "-c" || strings.HasPrefix(a, "--comment="):
			if strings.HasPrefix(a, "--comment=") {
				comment = strings.TrimPrefix(a, "--comment=")
			} else {
				i++
				if i >= len(args) {
					fatal("missing -c value")
				}
				comment = args[i]
			}
		case a == "-l" || strings.HasPrefix(a, "--piece-length="):
			val := ""
			if strings.HasPrefix(a, "--piece-length=") {
				val = strings.TrimPrefix(a, "--piece-length=")
			} else {
				i++
				if i >= len(args) {
					fatal("missing -l value")
				}
				val = args[i]
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				fatal("invalid -l value")
			}
			pieceLog = n
		case strings.HasPrefix(a, "-"):
			fatal("unknown option " + a)
		default:
			paths = append(paths, a)
		}
	}
	if !jsonMode {
		fatal("JSON flag required (--json|-j|-J) for Fleet OPEN-11")
	}
	if len(announce) == 0 || len(paths) == 0 {
		fatal("announce (-a) and at least one path are required")
	}
	pieceLen := 0
	if pieceLog > 0 {
		pieceLen = 1 << pieceLog
	}
	sum, err := metainfo.CreateTorrent(metainfo.CreateOptions{
		Paths:       paths,
		Announce:    announce,
		PieceLength: pieceLen,
		Private:     private,
		Name:        name,
		Comment:     comment,
		OutputPath:  outPath,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktorrent: %v\n", err)
		os.Exit(1)
	}
	out := map[string]any{
		"ok":       true,
		"path":     sum.Path,
		"name":     sum.Name,
		"infoHash": sum.InfoHash,
		"length":   sum.Length,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	os.Exit(2)
}
