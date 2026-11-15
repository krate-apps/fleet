// Package feeds builds ruTorrent-compatible torrent-list RSS documents.
package feeds

import (
	"encoding/xml"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/krate-apps/fleet/internal/rtorrent"
)

// Modes are the feed views ruTorrent plugins/feeds exposes.
var Modes = []string{"all", "downloading", "completed", "active", "inactive", "error"}

// ModeTitle is a human label for a mode.
func ModeTitle(mode string) string {
	switch mode {
	case "all":
		return "All torrents"
	case "downloading":
		return "Downloading"
	case "completed":
		return "Completed"
	case "active":
		return "Active"
	case "inactive":
		return "Inactive"
	case "error":
		return "Error"
	default:
		return mode
	}
}

// ValidMode reports whether mode is supported.
func ValidMode(mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	for _, m := range Modes {
		if m == mode {
			return true
		}
	}
	return false
}

// Filter returns torrents matching mode (ruTorrent feeds semantics).
func Filter(list []rtorrent.Torrent, mode string) []rtorrent.Torrent {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "all"
	}
	out := make([]rtorrent.Torrent, 0, len(list))
	for _, t := range list {
		switch mode {
		case "all":
			out = append(out, t)
		case "downloading":
			if !t.Complete {
				out = append(out, t)
			}
		case "completed":
			if t.Complete {
				out = append(out, t)
			}
		case "active":
			if t.DownRate >= 1024 || t.UpRate >= 1024 {
				out = append(out, t)
			}
		case "inactive":
			if t.DownRate < 1024 && t.UpRate < 1024 {
				out = append(out, t)
			}
		case "error":
			if strings.EqualFold(t.State, "error") {
				out = append(out, t)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ai, aj := out[i].FinishedAt, out[j].FinishedAt
		if ai == 0 {
			ai = out[i].StartedAt
		}
		if aj == 0 {
			aj = out[j].StartedAt
		}
		if ai != aj {
			return ai > aj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// BuildOptions configures the RSS channel.
type BuildOptions struct {
	Mode        string
	SelfLink    string
	ServerName  string
	Description string
	IncludeDesc bool
}

// BuildRSS returns an RSS 2.0 document for the filtered torrent list.
func BuildRSS(list []rtorrent.Torrent, opts BuildOptions) ([]byte, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "all"
	}
	if !ValidMode(mode) {
		return nil, fmt.Errorf("unknown feed mode %q", mode)
	}
	items := Filter(list, mode)
	server := strings.TrimSpace(opts.ServerName)
	if server == "" {
		server = "Fleet"
	}
	title := server + ": " + ModeTitle(mode)
	desc := strings.TrimSpace(opts.Description)
	if desc == "" {
		desc = "*** Fleet torrent feed ***"
	}

	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<rss version="2.0"><channel>`)
	b.WriteString("<title>" + xmlEscape(title) + "</title>")
	b.WriteString("<description>" + xmlEscape(desc) + "</description>")
	if link := strings.TrimSpace(opts.SelfLink); link != "" {
		b.WriteString("<link>" + xmlEscape(link) + "</link>")
	}
	for _, t := range items {
		b.WriteString("<item>")
		b.WriteString("<title>" + xmlEscape(t.Name) + "</title>")
		b.WriteString(`<guid isPermaLink="false">` + xmlEscape(t.Hash) + "</guid>")
		if t.Label != "" {
			b.WriteString("<category>" + xmlEscape(t.Label) + "</category>")
		}
		if ts := itemTime(t); ts > 0 {
			b.WriteString("<pubDate>" + xmlEscape(time.Unix(ts, 0).UTC().Format(time.RFC1123Z)) + "</pubDate>")
		}
		if opts.IncludeDesc {
			b.WriteString("<description><![CDATA[" + itemDescription(t) + "]]></description>")
		}
		b.WriteString("</item>")
	}
	b.WriteString("</channel></rss>")
	return []byte(b.String()), nil
}

func itemTime(t rtorrent.Torrent) int64 {
	if t.FinishedAt > 0 {
		return t.FinishedAt
	}
	return t.StartedAt
}

func itemDescription(t rtorrent.Torrent) string {
	var b strings.Builder
	b.WriteString("<p><strong>Size:</strong> " + html.EscapeString(formatBytes(t.SizeBytes)) + "<br>")
	b.WriteString("<strong>Downloaded:</strong> " + html.EscapeString(formatBytes(t.BytesDone)) + "<br>")
	b.WriteString("<strong>Uploaded:</strong> " + html.EscapeString(formatBytes(t.UpTotal)) + "<br>")
	b.WriteString(fmt.Sprintf("<strong>Ratio:</strong> %.3f<br>", t.Ratio))
	b.WriteString("<strong>State:</strong> " + html.EscapeString(t.State) + "<br>")
	if t.DownRate > 0 {
		b.WriteString("<strong>Down:</strong> " + html.EscapeString(formatBytes(t.DownRate)+"/s") + "<br>")
	}
	if t.UpRate > 0 {
		b.WriteString("<strong>Up:</strong> " + html.EscapeString(formatBytes(t.UpRate)+"/s") + "<br>")
	}
	if t.ETASeconds > 0 {
		b.WriteString(fmt.Sprintf("<strong>ETA:</strong> %ds<br>", t.ETASeconds))
	}
	b.WriteString("</p>")
	return b.String()
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
