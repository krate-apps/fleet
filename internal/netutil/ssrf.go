// Package netutil provides shared network safety helpers (SSRF guards).
package netutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSafeRedirects = 5

// BlockedIP reports whether ip must not be contacted for operator-initiated HTTP.
func BlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// CGNAT / benchmarking / documentation ranges often abused for SSRF.
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127: // 100.64.0.0/10
			return true
		case ip4[0] == 169 && ip4[1] == 254: // link-local / cloud metadata
			return true
		case ip4[0] == 0:
			return true
		}
	}
	return false
}

// ValidatePublicURL ensures scheme/host look safe before dial (DNS re-checked in dialer).
func ValidatePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("only http(s) URLs are allowed")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL host required")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL host required")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" {
		return nil, fmt.Errorf("host %q is not allowed", host)
	}
	if ip := net.ParseIP(host); ip != nil && BlockedIP(ip) {
		return nil, fmt.Errorf("host %q resolves to a blocked address", host)
	}
	return u, nil
}

// SafeHTTPClient returns an HTTP client that blocks private/link-local targets and re-validates redirects.
func SafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				host = address
				port = ""
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var last error
			for _, ipa := range ips {
				if BlockedIP(ipa.IP) {
					last = fmt.Errorf("blocked address %s for host %s", ipa.IP, host)
					continue
				}
				addr := ipa.IP.String()
				if ipa.IP.To4() == nil {
					addr = "[" + addr + "]"
				}
				if port != "" {
					addr = net.JoinHostPort(ipa.IP.String(), port)
				}
				conn, err := dialer.DialContext(ctx, network, addr)
				if err != nil {
					last = err
					continue
				}
				return conn, nil
			}
			if last == nil {
				last = fmt.Errorf("no safe addresses for host %s", host)
			}
			return nil, last
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxSafeRedirects {
				return fmt.Errorf("too many redirects")
			}
			if _, err := ValidatePublicURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}
