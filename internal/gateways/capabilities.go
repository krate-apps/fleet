package gateways

// Capabilities describes what Fleet can do against a remote client kind (PD-GW-03).
type Capabilities struct {
	List         bool `json:"list"`
	Add          bool `json:"add"`
	Label        bool `json:"label"`
	StartStop    bool `json:"startStop"`
	Erase        bool `json:"erase"`
	EraseData    bool `json:"eraseData"`
	Reannounce   bool `json:"reannounce"`
	Files        bool `json:"files"`
	Trackers     bool `json:"trackers"`
	Peers        bool `json:"peers"`
	Chunks       bool `json:"chunks"`
	TorrentRates bool `json:"torrentRates"`
}

// CapabilitiesFor returns the static matrix for a client kind.
func CapabilitiesFor(kind Kind) Capabilities {
	kind = Kind(normalizeKind(string(kind)))
	switch kind {
	case KindRTorrent:
		return Capabilities{
			List: true, Add: true, Label: true, StartStop: true,
			Erase: true, EraseData: true, Reannounce: true,
			Files: true, Trackers: true, Peers: true, Chunks: true,
			TorrentRates: true,
		}
	case KindQBittorrent:
		return Capabilities{
			List: true, Add: true, Label: true, StartStop: true,
			Erase: true, EraseData: true, Reannounce: true,
			Files: true, Trackers: true, Peers: true, Chunks: false,
			TorrentRates: true,
		}
	case KindDeluge:
		return Capabilities{
			List: true, Add: true, Label: true, StartStop: true,
			Erase: true, EraseData: true, Reannounce: true,
			Files: true, Trackers: true, Peers: false, Chunks: false,
			TorrentRates: true,
		}
	case KindTransmission:
		return Capabilities{
			List: true, Add: true, Label: true, StartStop: true,
			Erase: true, EraseData: true, Reannounce: true,
			Files: true, Trackers: true, Peers: true, Chunks: false,
			TorrentRates: true,
		}
	default:
		return Capabilities{}
	}
}

func normalizeKind(s string) string {
	switch s {
	case "qbittorrent", "qbit", "qb":
		return string(KindQBittorrent)
	case "deluge":
		return string(KindDeluge)
	case "transmission":
		return string(KindTransmission)
	case "rtorrent":
		return string(KindRTorrent)
	default:
		return s
	}
}

// SupportsFeature reports whether the kind supports a named feature key.
func SupportsFeature(kind Kind, feature string) bool {
	c := CapabilitiesFor(kind)
	switch feature {
	case "list":
		return c.List
	case "add":
		return c.Add
	case "label":
		return c.Label
	case "startStop", "start", "stop", "pause", "resume":
		return c.StartStop
	case "erase":
		return c.Erase
	case "erase_data", "eraseData":
		return c.EraseData
	case "reannounce":
		return c.Reannounce
	case "files":
		return c.Files
	case "trackers":
		return c.Trackers
	case "peers":
		return c.Peers
	case "chunks":
		return c.Chunks
	case "torrentRates", "rates", "bandwidth":
		return c.TorrentRates
	default:
		return false
	}
}
