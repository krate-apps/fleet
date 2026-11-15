// Package geoip optionally enriches peer addresses with ISO country codes
// from a MaxMind GeoLite2/GeoIP2 Country MMDB (ruTorrent geoip parity).
package geoip

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// DB wraps a MaxMind Country database. Nil-safe: methods on a nil *DB do nothing.
type DB struct {
	mu     sync.RWMutex
	reader *maxminddb.Reader
	path   string
}

type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Open loads a GeoLite2-Country (or GeoIP2-Country) MMDB from path.
// Empty path returns (nil, nil) — callers treat that as geoip disabled.
func Open(path string) (*DB, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("geoip open %q: %w", path, err)
	}
	return &DB{reader: r, path: path}, nil
}

// Close releases the MMDB handle.
func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reader == nil {
		return nil
	}
	err := d.reader.Close()
	d.reader = nil
	return err
}

// Enabled reports whether a database is loaded.
func (d *DB) Enabled() bool {
	if d == nil {
		return false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.reader != nil
}

// Path returns the configured database path (may be empty).
func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// CountryISO returns a two-letter ISO country code for addr, or "" if unknown.
// addr may be a bare IP or host:port (IPv4/IPv6).
func (d *DB) CountryISO(addr string) string {
	if d == nil {
		return ""
	}
	ip := parseIP(addr)
	if ip == nil {
		return ""
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.reader == nil {
		return ""
	}
	var rec countryRecord
	if err := d.reader.Lookup(ip, &rec); err != nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(rec.Country.ISOCode))
}

func parseIP(addr string) net.IP {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	} else if strings.HasPrefix(addr, "[") && strings.Contains(addr, "]") {
		// bare [ipv6] without port
		if end := strings.IndexByte(addr, ']'); end > 1 {
			addr = addr[1:end]
		}
	}
	return net.ParseIP(addr)
}
