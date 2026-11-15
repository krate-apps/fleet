package rtorrent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// GlobalRates are rTorrent global throttle limits in bytes/sec (0 = unlimited).
type GlobalRates struct {
	DownRate int64 `json:"downRate"`
	UpRate   int64 `json:"upRate"`
}

// GetGlobalRates reads throttle.global_{down,up}.max_rate.
func (c *Client) GetGlobalRates(ctx context.Context) (GlobalRates, error) {
	down, err := c.Call(ctx, "throttle.global_down.max_rate")
	if err != nil {
		return GlobalRates{}, fmt.Errorf("global down rate: %w", err)
	}
	up, err := c.Call(ctx, "throttle.global_up.max_rate")
	if err != nil {
		return GlobalRates{}, fmt.Errorf("global up rate: %w", err)
	}
	d, _ := down.AsInt()
	u, _ := up.AsInt()
	return GlobalRates{DownRate: d, UpRate: u}, nil
}

// SetGlobalRates sets global download/upload rate caps (bytes/sec; 0 unlimited).
func (c *Client) SetGlobalRates(ctx context.Context, downRate, upRate int64) error {
	if downRate < 0 || upRate < 0 {
		return fmt.Errorf("rates must be >= 0")
	}
	if _, err := c.Call(ctx, "throttle.global_down.max_rate.set", "", downRate); err != nil {
		// Some builds omit the empty target argument.
		if _, err2 := c.Call(ctx, "throttle.global_down.max_rate.set", downRate); err2 != nil {
			return fmt.Errorf("set down rate: %v / %v", err, err2)
		}
	}
	if _, err := c.Call(ctx, "throttle.global_up.max_rate.set", "", upRate); err != nil {
		if _, err2 := c.Call(ctx, "throttle.global_up.max_rate.set", upRate); err2 != nil {
			return fmt.Errorf("set up rate: %v / %v", err, err2)
		}
	}
	return nil
}

// EnsureNamedThrottle creates/updates a named throttle profile (bytes/sec; 0 = unlimited).
// Uses throttle.up / throttle.down with an empty target (rTorrent 0.15+).
func (c *Client) EnsureNamedThrottle(ctx context.Context, name string, upRate, downRate int64) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("throttle name required")
	}
	if upRate < 0 || downRate < 0 {
		return fmt.Errorf("rates must be >= 0")
	}
	if _, err := c.Call(ctx, "throttle.up", "", name, strconv.FormatInt(upRate, 10)); err != nil {
		return fmt.Errorf("throttle.up %q: %w", name, err)
	}
	if _, err := c.Call(ctx, "throttle.down", "", name, strconv.FormatInt(downRate, 10)); err != nil {
		return fmt.Errorf("throttle.down %q: %w", name, err)
	}
	return nil
}

// SetTorrentThrottleName assigns a named throttle to a torrent (empty name clears).
// Active torrents are briefly stopped so rTorrent reloads the throttle binding.
func (c *Client) SetTorrentThrottleName(ctx context.Context, hash, name string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if hash == "" {
		return fmt.Errorf("hash required")
	}
	name = strings.TrimSpace(name)

	active := false
	if v, err := c.Call(ctx, "d.is_active", hash); err == nil {
		if n, err := v.AsInt(); err == nil {
			active = n != 0
		}
	}
	if active {
		if _, err := c.Call(ctx, "d.stop", hash); err != nil {
			return fmt.Errorf("stop before throttle: %w", err)
		}
	}
	if _, err := c.Call(ctx, "d.throttle_name.set", hash, name); err != nil {
		if active {
			_, _ = c.Call(ctx, "d.start", hash)
		}
		return fmt.Errorf("d.throttle_name.set: %w", err)
	}
	if active {
		if _, err := c.Call(ctx, "d.start", hash); err != nil {
			return fmt.Errorf("restart after throttle: %w", err)
		}
	}
	return nil
}
