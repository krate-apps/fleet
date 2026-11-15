package rtorrent

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// AddTracker inserts a tracker URL into group 0.
func (c *Client) AddTracker(ctx context.Context, hash, trackerURL string) error {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	trackerURL = strings.TrimSpace(trackerURL)
	if hash == "" {
		return fmt.Errorf("empty hash")
	}
	if err := validateTrackerURL(trackerURL); err != nil {
		return err
	}
	_, err := c.Call(ctx, "d.tracker.insert", hash, int64(0), trackerURL)
	return err
}

// SetTrackerEnabled enables or disables a tracker by id (HASH:tN).
func (c *Client) SetTrackerEnabled(ctx context.Context, trackerID string, enabled bool) error {
	trackerID = strings.TrimSpace(trackerID)
	if trackerID == "" {
		return fmt.Errorf("empty tracker id")
	}
	flag := int64(0)
	if enabled {
		flag = 1
	}
	_, err := c.Call(ctx, "t.is_enabled.set", trackerID, flag)
	return err
}

// RemoveTracker erases a tracker by id (HASH:tN).
func (c *Client) RemoveTracker(ctx context.Context, trackerID string) error {
	trackerID = strings.TrimSpace(trackerID)
	if trackerID == "" {
		return fmt.Errorf("empty tracker id")
	}
	_, err := c.Call(ctx, "t.erase", trackerID)
	return err
}

func validateTrackerURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("empty tracker url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid tracker url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "udp":
	default:
		return fmt.Errorf("tracker scheme must be http, https, or udp")
	}
	if u.Host == "" {
		return fmt.Errorf("tracker url missing host")
	}
	return nil
}
