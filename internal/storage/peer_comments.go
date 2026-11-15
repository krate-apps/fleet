package storage

import (
	"fmt"
	"strings"
	"time"
)

// PeerComment is a user annotation for a peer address (ruTorrent geoip comments parity).
type PeerComment struct {
	Address   string `json:"address"`
	Comment   string `json:"comment"`
	UpdatedAt string `json:"updatedAt"`
}

// ListPeerComments returns comments keyed by peer address.
func (s *Store) ListPeerComments(addresses []string) (map[string]string, error) {
	out := make(map[string]string)
	if s == nil || s.db == nil || len(addresses) == 0 {
		return out, nil
	}
	uniq := make([]string, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, a := range addresses {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		uniq = append(uniq, a)
	}
	if len(uniq) == 0 {
		return out, nil
	}
	// SQLite has a practical bind limit; batch in chunks.
	const batch = 200
	for i := 0; i < len(uniq); i += batch {
		end := i + batch
		if end > len(uniq) {
			end = len(uniq)
		}
		chunk := uniq[i:end]
		placeholders := make([]string, len(chunk))
		args := make([]any, len(chunk))
		for j, a := range chunk {
			placeholders[j] = "?"
			args[j] = a
		}
		q := fmt.Sprintf(`SELECT address, comment FROM peer_comments WHERE address IN (%s)`, strings.Join(placeholders, ","))
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var addr, comment string
			if err := rows.Scan(&addr, &comment); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[addr] = comment
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SetPeerComment upserts or deletes a peer comment. Empty comment deletes the row.
func (s *Store) SetPeerComment(address, comment string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage unavailable")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("address required")
	}
	comment = strings.TrimSpace(comment)
	if len(comment) > 512 {
		return fmt.Errorf("comment too long (max 512)")
	}
	if comment == "" {
		_, err := s.db.Exec(`DELETE FROM peer_comments WHERE address = ?`, address)
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO peer_comments(address, comment, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(address) DO UPDATE SET comment = excluded.comment, updated_at = excluded.updated_at`,
		address, comment, now,
	)
	return err
}
