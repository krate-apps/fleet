// Package sysinfo provides host disk and load metrics for Fleet status (cpuload/diskspace parity).
package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DiskUsage is free/total space for one filesystem path.
type DiskUsage struct {
	Path      string  `json:"path"`
	Total     uint64  `json:"total"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"`
	UsedPct   float64 `json:"usedPct"`
}

// LoadAvg is the 1/5/15 minute load average.
type LoadAvg struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
	Procs  string  `json:"procs,omitempty"`
}

// DiskForPath returns Statfs-backed usage for path (must exist).
func DiskForPath(path string) (DiskUsage, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return DiskUsage{}, fmt.Errorf("path required")
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskUsage{}, err
	}
	total := st.Blocks * uint64(st.Bsize)
	free := st.Bfree * uint64(st.Bsize)
	avail := st.Bavail * uint64(st.Bsize)
	used := total - free
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return DiskUsage{
		Path:      path,
		Total:     total,
		Free:      free,
		Available: avail,
		Used:      used,
		UsedPct:   pct,
	}, nil
}

// DiskForRoots returns usage for each root (skips missing paths).
func DiskForRoots(roots []string) []DiskUsage {
	out := make([]DiskUsage, 0, len(roots))
	seen := map[string]struct{}{}
	for _, root := range roots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || root == "." {
			continue
		}
		u, err := DiskForPath(root)
		if err != nil {
			continue
		}
		// Deduplicate by device mount identity via path key after resolve.
		key := u.Path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, u)
	}
	return out
}

// Load returns /proc/loadavg when available.
func Load() (LoadAvg, error) {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return LoadAvg{}, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return LoadAvg{}, fmt.Errorf("empty loadavg")
	}
	parts := strings.Fields(sc.Text())
	if len(parts) < 3 {
		return LoadAvg{}, fmt.Errorf("invalid loadavg")
	}
	l1, err1 := strconv.ParseFloat(parts[0], 64)
	l5, err5 := strconv.ParseFloat(parts[1], 64)
	l15, err15 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err5 != nil || err15 != nil {
		return LoadAvg{}, fmt.Errorf("invalid loadavg numbers")
	}
	procs := ""
	if len(parts) >= 4 {
		procs = parts[3]
	}
	return LoadAvg{Load1: l1, Load5: l5, Load15: l15, Procs: procs}, nil
}
