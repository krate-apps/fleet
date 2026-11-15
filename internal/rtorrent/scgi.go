package rtorrent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDialTimeout = 3 * time.Second
	defaultIOTimeout   = 10 * time.Second
	maxResponseBytes   = 32 << 20 // 32 MiB
)

func roundTripSCGI(ctx context.Context, ep Endpoint, body []byte) ([]byte, error) {
	dialer := net.Dialer{Timeout: defaultDialTimeout}
	conn, err := dialer.DialContext(ctx, ep.Network, ep.Address)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", ep, err)
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultIOTimeout)
	}
	_ = conn.SetDeadline(deadline)

	payload := buildSCGIRequest(body)
	if _, err := conn.Write(payload); err != nil {
		return nil, fmt.Errorf("write scgi: %w", err)
	}

	raw, err := io.ReadAll(io.LimitReader(conn, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read scgi: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return nil, fmt.Errorf("scgi response exceeds %d bytes", maxResponseBytes)
	}
	return stripHTTPLikeHeaders(raw), nil
}

func buildSCGIRequest(body []byte) []byte {
	// Same header shape as Krate's DownloadClientStatsService probe.
	headers := strings.Join([]string{
		"CONTENT_LENGTH", strconv.Itoa(len(body)),
		"SCGI", "1",
		"REQUEST_METHOD", "POST",
		"",
	}, "\x00")
	var buf bytes.Buffer
	buf.Grow(len(headers) + len(body) + 32)
	buf.WriteString(strconv.Itoa(len(headers)))
	buf.WriteByte(':')
	buf.WriteString(headers)
	buf.WriteByte(',')
	buf.Write(body)
	return buf.Bytes()
}

func stripHTTPLikeHeaders(raw []byte) []byte {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return raw[i+4:]
	}
	if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
		return raw[i+2:]
	}
	return raw
}
