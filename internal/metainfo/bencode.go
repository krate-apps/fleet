package metainfo

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// Decode parses a bencoded value from data.
func Decode(data []byte) (any, error) {
	v, _, err := decodeValue(data)
	return v, err
}

func decodeValue(data []byte) (any, int, error) {
	if len(data) == 0 {
		return nil, 0, io.ErrUnexpectedEOF
	}
	switch data[0] {
	case 'i':
		end := bytes.IndexByte(data, 'e')
		if end < 1 {
			return nil, 0, fmt.Errorf("bad int")
		}
		n, err := strconv.ParseInt(string(data[1:end]), 10, 64)
		if err != nil {
			return nil, 0, err
		}
		return n, end + 1, nil
	case 'l':
		var list []any
		i := 1
		for i < len(data) && data[i] != 'e' {
			v, n, err := decodeValue(data[i:])
			if err != nil {
				return nil, 0, err
			}
			list = append(list, v)
			i += n
		}
		if i >= len(data) || data[i] != 'e' {
			return nil, 0, fmt.Errorf("unterminated list")
		}
		return list, i + 1, nil
	case 'd':
		dict := map[string]any{}
		i := 1
		for i < len(data) && data[i] != 'e' {
			k, n, err := decodeValue(data[i:])
			if err != nil {
				return nil, 0, err
			}
			ks, ok := k.(string)
			if !ok {
				return nil, 0, fmt.Errorf("dict key not string")
			}
			i += n
			v, n, err := decodeValue(data[i:])
			if err != nil {
				return nil, 0, err
			}
			dict[ks] = v
			i += n
		}
		if i >= len(data) || data[i] != 'e' {
			return nil, 0, fmt.Errorf("unterminated dict")
		}
		return dict, i + 1, nil
	default:
		if data[0] < '0' || data[0] > '9' {
			return nil, 0, fmt.Errorf("bad bencode type %q", data[0])
		}
		colon := bytes.IndexByte(data, ':')
		if colon < 1 {
			return nil, 0, fmt.Errorf("bad string")
		}
		ln, err := strconv.Atoi(string(data[:colon]))
		if err != nil || ln < 0 {
			return nil, 0, fmt.Errorf("bad string length")
		}
		start := colon + 1
		end := start + ln
		if end > len(data) {
			return nil, 0, io.ErrUnexpectedEOF
		}
		return string(data[start:end]), end, nil
	}
}

// Encode serializes v as bencode (maps keyed as string).
func Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeValue(w *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case string:
		fmt.Fprintf(w, "%d:%s", len(t), t)
	case []byte:
		fmt.Fprintf(w, "%d:", len(t))
		_, _ = w.Write(t)
	case int:
		fmt.Fprintf(w, "i%de", t)
	case int64:
		fmt.Fprintf(w, "i%de", t)
	case []any:
		_ = w.WriteByte('l')
		for _, item := range t {
			if err := encodeValue(w, item); err != nil {
				return err
			}
		}
		_ = w.WriteByte('e')
	case map[string]any:
		_ = w.WriteByte('d')
		keys := sortedKeys(t)
		for _, k := range keys {
			if err := encodeValue(w, k); err != nil {
				return err
			}
			if err := encodeValue(w, t[k]); err != nil {
				return err
			}
		}
		_ = w.WriteByte('e')
	default:
		return fmt.Errorf("unsupported bencode type %T", v)
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
