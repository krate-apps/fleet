package metainfo

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileEntry is one file inside a torrent.
type FileEntry struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
}

// Summary is JSON-friendly torrent metadata (OPEN-11 dump shape).
type Summary struct {
	Name         string      `json:"name"`
	InfoHash     string      `json:"infoHash"`
	PieceLength  int64       `json:"pieceLength"`
	Length       int64       `json:"length"`
	Private      bool        `json:"private,omitempty"`
	Announce     string      `json:"announce,omitempty"`
	AnnounceList [][]string  `json:"announceList,omitempty"`
	Comment      string      `json:"comment,omitempty"`
	Files        []FileEntry `json:"files,omitempty"`
	Path         string      `json:"path,omitempty"`
}

// CreateOptions builds a single- or multi-file torrent.
type CreateOptions struct {
	Paths       []string
	Announce    []string
	PieceLength int // bytes; 0 = auto (256 KiB)
	Private     bool
	Name        string
	Comment     string
	OutputPath  string
}

type fileJob struct {
	abs    string
	rel    string
	length int64
}

// ParseTorrentFile reads a .torrent and returns a summary + raw infohash.
func ParseTorrentFile(path string) (*Summary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := ParseTorrentBytes(data)
	if err != nil {
		return nil, err
	}
	s.Path = path
	return s, nil
}

// ParseTorrentBytes parses a .torrent payload and returns a summary + infohash.
func ParseTorrentBytes(data []byte) (*Summary, error) {
	root, err := Decode(data)
	if err != nil {
		return nil, err
	}
	dict, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("torrent root is not a dict")
	}
	infoRaw, ok := dict["info"]
	if !ok {
		return nil, fmt.Errorf("missing info dict")
	}
	infoBytes, err := Encode(infoRaw)
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum(infoBytes)
	info, _ := infoRaw.(map[string]any)
	s := &Summary{
		InfoHash: strings.ToUpper(hex.EncodeToString(sum[:])),
	}
	if info != nil {
		if n, ok := info["name"].(string); ok {
			s.Name = n
		}
		if pl, ok := asInt(info["piece length"]); ok {
			s.PieceLength = pl
		}
		if _, ok := info["private"]; ok {
			if p, ok := asInt(info["private"]); ok && p != 0 {
				s.Private = true
			}
		}
		if length, ok := asInt(info["length"]); ok {
			s.Length = length
			s.Files = []FileEntry{{Path: s.Name, Length: length}}
		} else if files, ok := info["files"].([]any); ok {
			var total int64
			for _, f := range files {
				fm, _ := f.(map[string]any)
				if fm == nil {
					continue
				}
				ln, _ := asInt(fm["length"])
				var parts []string
				if pathList, ok := fm["path"].([]any); ok {
					for _, p := range pathList {
						if ps, ok := p.(string); ok {
							parts = append(parts, ps)
						}
					}
				}
				total += ln
				s.Files = append(s.Files, FileEntry{Path: filepath.Join(parts...), Length: ln})
			}
			s.Length = total
		}
	}
	if a, ok := dict["announce"].(string); ok {
		s.Announce = a
	}
	if c, ok := dict["comment"].(string); ok {
		s.Comment = c
	}
	if al, ok := dict["announce-list"].([]any); ok {
		for _, tier := range al {
			var urls []string
			if list, ok := tier.([]any); ok {
				for _, u := range list {
					if us, ok := u.(string); ok {
						urls = append(urls, us)
					}
				}
			}
			if len(urls) > 0 {
				s.AnnounceList = append(s.AnnounceList, urls)
			}
		}
	}
	return s, nil
}

// CreateTorrent writes a .torrent for the given paths and returns a summary.
func CreateTorrent(opts CreateOptions) (*Summary, error) {
	if len(opts.Paths) == 0 {
		return nil, fmt.Errorf("paths required")
	}
	if len(opts.Announce) == 0 {
		return nil, fmt.Errorf("announce required")
	}
	pieceLen := opts.PieceLength
	if pieceLen <= 0 {
		pieceLen = 256 * 1024
	}
	if pieceLen&(pieceLen-1) != 0 {
		return nil, fmt.Errorf("pieceLength must be a power of two")
	}

	var jobs []fileJob
	var total int64
	name := strings.TrimSpace(opts.Name)

	for _, p := range opts.Paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if st.IsDir() {
			if name == "" {
				name = filepath.Base(abs)
			}
			err := filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(abs, path)
				if err != nil {
					return err
				}
				jobs = append(jobs, fileJob{abs: path, rel: rel, length: info.Size()})
				total += info.Size()
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			if name == "" {
				name = filepath.Base(abs)
			}
			jobs = append(jobs, fileJob{abs: abs, rel: filepath.Base(abs), length: st.Size()})
			total += st.Size()
		}
	}
	if name == "" {
		name = "torrent"
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("no files to hash")
	}

	pieces, err := hashPieces(jobs, pieceLen)
	if err != nil {
		return nil, err
	}

	info := map[string]any{
		"name":         name,
		"piece length": int64(pieceLen),
		"pieces":       string(pieces),
	}
	if opts.Private {
		info["private"] = int64(1)
	}
	singleFile := false
	if len(opts.Paths) == 1 {
		if st, err := os.Stat(opts.Paths[0]); err == nil && !st.IsDir() {
			singleFile = true
			info["length"] = jobs[0].length
		}
	}
	if !singleFile {
		var files []any
		for _, j := range jobs {
			parts := strings.Split(filepath.ToSlash(j.rel), "/")
			pathVals := make([]any, len(parts))
			for i, p := range parts {
				pathVals[i] = p
			}
			files = append(files, map[string]any{
				"length": j.length,
				"path":   pathVals,
			})
		}
		info["files"] = files
	}

	root := map[string]any{
		"announce": opts.Announce[0],
		"info":     info,
	}
	if len(opts.Announce) > 1 {
		var tiers []any
		for _, a := range opts.Announce {
			tiers = append(tiers, []any{a})
		}
		root["announce-list"] = tiers
	}
	if strings.TrimSpace(opts.Comment) != "" {
		root["comment"] = opts.Comment
	}

	raw, err := Encode(root)
	if err != nil {
		return nil, err
	}
	out := opts.OutputPath
	if out == "" {
		out = name + ".torrent"
	}
	if err := os.WriteFile(out, raw, 0o600); err != nil {
		return nil, err
	}
	sum, err := ParseTorrentFile(out)
	if err != nil {
		return nil, err
	}
	return sum, nil
}

func hashPieces(jobs []fileJob, pieceLen int) ([]byte, error) {
	h := sha1.New()
	var pieces []byte
	var inPiece int
	flush := func() {
		if inPiece == 0 {
			return
		}
		pieces = append(pieces, h.Sum(nil)...)
		h.Reset()
		inPiece = 0
	}
	buf := make([]byte, 32*1024)
	for _, j := range jobs {
		f, err := os.Open(j.abs)
		if err != nil {
			return nil, err
		}
		for {
			n, err := f.Read(buf)
			if n > 0 {
				off := 0
				for off < n {
					need := pieceLen - inPiece
					take := n - off
					if take > need {
						take = need
					}
					_, _ = h.Write(buf[off : off+take])
					inPiece += take
					off += take
					if inPiece == pieceLen {
						flush()
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = f.Close()
				return nil, err
			}
		}
		_ = f.Close()
	}
	flush()
	return pieces, nil
}

func asInt(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	default:
		return 0, false
	}
}
