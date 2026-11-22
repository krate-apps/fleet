package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// validatePrivateBaseURL allows only http(s) to loopback or RFC1918 / ULA hosts.
// Link-local and cloud metadata hosts are refused (SSRF guard for worker-direct dial).
func validatePrivateBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("base URL must be http or https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("base URL host required")
	}
	if host == "metadata.google.internal" || strings.HasSuffix(host, ".metadata.google.internal") {
		return nil, fmt.Errorf("host %q is not allowed", host)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return u, nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if !allowedPrivateIP(ip) {
			return nil, fmt.Errorf("host %q is not loopback/private", host)
		}
		return u, nil
	}
	// Resolve hostname — all answers must be private/loopback.
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q has no addresses", host)
	}
	for _, ip := range ips {
		if !allowedPrivateIP(ip) {
			return nil, fmt.Errorf("host %q resolves to non-private %s", host, ip)
		}
	}
	return u, nil
}

func allowedPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		// Block CGNAT and AWS/GCP metadata-ish ranges abused for SSRF.
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
		if ip4[0] == 169 && ip4[1] == 254 {
			return false
		}
	}
	return ip.IsPrivate()
}

func joinAPI(base *url.URL, path string) string {
	u := *base
	basePath := strings.TrimRight(u.Path, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u.Path = basePath + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
