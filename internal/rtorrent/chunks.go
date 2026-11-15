package rtorrent

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

// ChunkMap is the piece bitfield for a torrent (ruTorrent chunks plugin).
type ChunkMap struct {
	Hash       string `json:"hash"`
	Bitfield   string `json:"bitfield"` // lowercase hex from rTorrent
	ChunkSize  int64  `json:"chunkSize"`
	SizeChunks int64  `json:"sizeChunks"`
	Completed  int64  `json:"completed"`
}

// GetChunks reads d.bitfield / d.chunk_size / d.size_chunks for hash.
func (c *Client) GetChunks(ctx context.Context, hash string) (ChunkMap, error) {
	hash = strings.ToUpper(strings.TrimSpace(hash))
	if len(hash) != 40 {
		return ChunkMap{}, fmt.Errorf("invalid torrent hash")
	}

	bitfield, err := c.callStringFallback(ctx, hash, "d.bitfield", "d.get_bitfield")
	if err != nil {
		return ChunkMap{}, err
	}
	chunkSize, err := c.callIntFallback(ctx, hash, "d.chunk_size", "d.get_chunk_size")
	if err != nil {
		return ChunkMap{}, err
	}
	sizeChunks, err := c.callIntFallback(ctx, hash, "d.size_chunks", "d.get_size_chunks")
	if err != nil {
		return ChunkMap{}, err
	}

	bf := strings.ToLower(strings.TrimSpace(bitfield))
	completed := countBitfieldOnes(bf, sizeChunks)
	return ChunkMap{
		Hash:       hash,
		Bitfield:   bf,
		ChunkSize:  chunkSize,
		SizeChunks: sizeChunks,
		Completed:  completed,
	}, nil
}

func (c *Client) callStringFallback(ctx context.Context, hash string, methods ...string) (string, error) {
	var last error
	for _, m := range methods {
		v, err := c.Call(ctx, m, hash)
		if err != nil {
			last = err
			continue
		}
		return strings.TrimSpace(v.AsString()), nil
	}
	if last != nil {
		return "", last
	}
	return "", fmt.Errorf("no method succeeded")
}

func (c *Client) callIntFallback(ctx context.Context, hash string, methods ...string) (int64, error) {
	var last error
	for _, m := range methods {
		v, err := c.Call(ctx, m, hash)
		if err != nil {
			last = err
			continue
		}
		n, err := v.AsInt()
		if err != nil {
			last = err
			continue
		}
		return n, nil
	}
	if last != nil {
		return 0, last
	}
	return 0, fmt.Errorf("no method succeeded")
}

// countBitfieldOnes counts set bits in a hex bitfield, capped to sizeChunks when > 0.
func countBitfieldOnes(hexStr string, sizeChunks int64) int64 {
	hexStr = strings.TrimSpace(hexStr)
	if hexStr == "" {
		return 0
	}
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return 0
	}
	var n int64
	limit := sizeChunks
	var bitIdx int64
	for _, b := range raw {
		for i := 7; i >= 0; i-- {
			if limit > 0 && bitIdx >= limit {
				return n
			}
			if b&(1<<uint(i)) != 0 {
				n++
			}
			bitIdx++
		}
	}
	return n
}
