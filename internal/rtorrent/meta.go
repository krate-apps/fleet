package rtorrent

import (
	"context"
	"fmt"
	"strings"
)

// SetLabel sets d.custom1 (the conventional label field used by ruTorrent/Flood).
func (c *Client) SetLabel(ctx context.Context, hash, label string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	_, err := c.Call(ctx, "d.custom1.set", hash, label)
	return err
}

// SetDirectory sets d.directory after a data move (best-effort on some builds).
func (c *Client) SetDirectory(ctx context.Context, hash, directory string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	directory = strings.TrimSpace(directory)
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	if directory == "" {
		return fmt.Errorf("empty directory")
	}
	_, err := c.Call(ctx, "d.directory.set", hash, directory)
	return err
}

// SetName sets d.name (display name; ruTorrent rename-torrent parity).
func (c *Client) SetName(ctx context.Context, hash, name string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	name = strings.TrimSpace(name)
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	if name == "" {
		return fmt.Errorf("empty name")
	}
	if len(name) > 1024 {
		return fmt.Errorf("name too long")
	}
	_, err := c.Call(ctx, "d.name.set", hash, name)
	return err
}

// SetTorrentPriority sets d.priority (0..3 typically).
func (c *Client) SetTorrentPriority(ctx context.Context, hash string, priority int64) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	if priority < 0 || priority > 3 {
		return fmt.Errorf("priority must be 0..3")
	}
	_, err := c.Call(ctx, "d.priority.set", hash, priority)
	return err
}

// SetFilePriority sets f.priority for one file index (0=off, 1=normal, 2=high).
func (c *Client) SetFilePriority(ctx context.Context, hash string, index, priority int64) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	if index < 0 {
		return fmt.Errorf("invalid file index")
	}
	if priority < 0 || priority > 2 {
		return fmt.Errorf("file priority must be 0..2")
	}
	_, err := c.Call(ctx, "f.priority.set", fmt.Sprintf("%s:f%d", hash, index), priority)
	if err != nil {
		// Alternate signature used by some builds: (hash, index, priority)
		_, err = c.Call(ctx, "f.priority.set", hash, index, priority)
	}
	return err
}

// SetFilePriorities applies the same priority to multiple indexes.
func (c *Client) SetFilePriorities(ctx context.Context, hash string, indexes []int64, priority int64) error {
	for _, idx := range indexes {
		if err := c.SetFilePriority(ctx, hash, idx, priority); err != nil {
			return fmt.Errorf("file %d: %w", idx, err)
		}
	}
	// Ask rTorrent to update the bitfield selection after priority changes.
	_, _ = c.Call(ctx, "d.update_priorities", strings.ToUpper(strings.TrimSpace(hash)))
	return nil
}
