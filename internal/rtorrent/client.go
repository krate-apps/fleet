package rtorrent

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"
)

// Client talks to one rTorrent instance over SCGI/XML-RPC.
type Client struct {
	ep      Endpoint
	mu      sync.Mutex
	timeout time.Duration
}

// NewClient builds a client for the given address string.
func NewClient(address string) (*Client, error) {
	ep, err := ParseAddress(address)
	if err != nil {
		return nil, err
	}
	return &Client{ep: ep, timeout: defaultIOTimeout}, nil
}

// Endpoint returns the resolved endpoint.
func (c *Client) Endpoint() Endpoint {
	return c.ep
}

// Call invokes a single XML-RPC method.
func (c *Client) Call(ctx context.Context, method string, params ...any) (Value, error) {
	body, err := encodeCall(method, params)
	if err != nil {
		return Value{}, err
	}

	callCtx := ctx
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := roundTripSCGI(callCtx, c.ep, body)
	if err != nil {
		return Value{}, err
	}
	return decodeResponse(bytes.NewReader(raw))
}

// CallString is a convenience for methods returning a string-like value.
func (c *Client) CallString(ctx context.Context, method string, params ...any) (string, error) {
	v, err := c.Call(ctx, method, params...)
	if err != nil {
		return "", err
	}
	return v.AsString(), nil
}

// Ping checks that the engine answers a basic RPC.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Call(ctx, "system.client_version")
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}
