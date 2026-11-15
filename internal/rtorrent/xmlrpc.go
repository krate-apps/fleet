package rtorrent

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Value is a decoded XML-RPC value (subset used by rTorrent).
type Value struct {
	Kind   string // string, int, i8, bool, double, array, struct, nil
	Str    string
	Int    int64
	Bool   bool
	Float  float64
	Array  []Value
	Struct map[string]Value
}

func (v Value) AsString() string {
	switch v.Kind {
	case "string":
		return v.Str
	case "int", "i8":
		return strconv.FormatInt(v.Int, 10)
	case "bool":
		if v.Bool {
			return "1"
		}
		return "0"
	case "double":
		return strconv.FormatFloat(v.Float, 'f', -1, 64)
	default:
		return v.Str
	}
}

func (v Value) AsInt() (int64, error) {
	switch v.Kind {
	case "int", "i8":
		return v.Int, nil
	case "string":
		return strconv.ParseInt(strings.TrimSpace(v.Str), 10, 64)
	case "bool":
		if v.Bool {
			return 1, nil
		}
		return 0, nil
	case "double":
		return int64(v.Float), nil
	default:
		return 0, fmt.Errorf("value kind %q is not numeric", v.Kind)
	}
}

func encodeCall(method string, params []any) ([]byte, error) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><methodCall><methodName>`)
	b.WriteString(xmlEscape(method))
	b.WriteString(`</methodName><params>`)
	for _, p := range params {
		b.WriteString(`<param><value>`)
		if err := writeValue(&b, p); err != nil {
			return nil, err
		}
		b.WriteString(`</value></param>`)
	}
	b.WriteString(`</params></methodCall>`)
	return []byte(b.String()), nil
}

func writeValue(b *strings.Builder, v any) error {
	switch t := v.(type) {
	case nil:
		b.WriteString(`<nil/>`)
	case string:
		b.WriteString(`<string>`)
		b.WriteString(xmlEscape(t))
		b.WriteString(`</string>`)
	case []byte:
		b.WriteString(`<base64>`)
		b.WriteString(base64.StdEncoding.EncodeToString(t))
		b.WriteString(`</base64>`)
	case int:
		b.WriteString(`<i4>`)
		b.WriteString(strconv.Itoa(t))
		b.WriteString(`</i4>`)
	case int32:
		b.WriteString(`<i4>`)
		b.WriteString(strconv.FormatInt(int64(t), 10))
		b.WriteString(`</i4>`)
	case int64:
		b.WriteString(`<i8>`)
		b.WriteString(strconv.FormatInt(t, 10))
		b.WriteString(`</i8>`)
	case bool:
		b.WriteString(`<boolean>`)
		if t {
			b.WriteString(`1`)
		} else {
			b.WriteString(`0`)
		}
		b.WriteString(`</boolean>`)
	case float64:
		b.WriteString(`<double>`)
		b.WriteString(strconv.FormatFloat(t, 'f', -1, 64))
		b.WriteString(`</double>`)
	case []any:
		b.WriteString(`<array><data>`)
		for _, item := range t {
			b.WriteString(`<value>`)
			if err := writeValue(b, item); err != nil {
				return err
			}
			b.WriteString(`</value>`)
		}
		b.WriteString(`</data></array>`)
	case map[string]any:
		b.WriteString(`<struct>`)
		for k, item := range t {
			b.WriteString(`<member><name>`)
			b.WriteString(xmlEscape(k))
			b.WriteString(`</name><value>`)
			if err := writeValue(b, item); err != nil {
				return err
			}
			b.WriteString(`</value></member>`)
		}
		b.WriteString(`</struct>`)
	default:
		return fmt.Errorf("unsupported xml-rpc type %T", v)
	}
	return nil
}

func xmlEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

type methodResponse struct {
	Fault  *faultBody  `xml:"fault"`
	Params *paramsBody `xml:"params"`
}

type faultBody struct {
	Value xmlValue `xml:"value"`
}

type paramsBody struct {
	Param []struct {
		Value xmlValue `xml:"value"`
	} `xml:"param"`
}

type xmlValue struct {
	String   *string     `xml:"string"`
	I4       *string     `xml:"i4"`
	Int      *string     `xml:"int"`
	I8       *string     `xml:"i8"`
	Boolean  *string     `xml:"boolean"`
	Double   *string     `xml:"double"`
	Nil      *struct{}   `xml:"nil"`
	Array    *xmlArray   `xml:"array"`
	Struct   *xmlStruct  `xml:"struct"`
	InnerXML string      `xml:",innerxml"`
}

type xmlArray struct {
	Data struct {
		Values []xmlValue `xml:"value"`
	} `xml:"data"`
}

type xmlStruct struct {
	Members []struct {
		Name  string   `xml:"name"`
		Value xmlValue `xml:"value"`
	} `xml:"member"`
}

func decodeResponse(r io.Reader) (Value, error) {
	var resp methodResponse
	dec := xml.NewDecoder(r)
	dec.Strict = false
	if err := dec.Decode(&resp); err != nil {
		return Value{}, fmt.Errorf("decode xml-rpc: %w", err)
	}
	if resp.Fault != nil {
		fault, err := resp.Fault.Value.toValue()
		if err != nil {
			return Value{}, fmt.Errorf("xml-rpc fault")
		}
		msg := faultMessage(fault)
		return Value{}, fmt.Errorf("xml-rpc fault: %s", msg)
	}
	if resp.Params == nil || len(resp.Params.Param) == 0 {
		return Value{Kind: "nil"}, nil
	}
	return resp.Params.Param[0].Value.toValue()
}

func faultMessage(v Value) string {
	if v.Kind == "struct" {
		if m, ok := v.Struct["faultString"]; ok {
			return m.AsString()
		}
	}
	return v.AsString()
}

func (v xmlValue) toValue() (Value, error) {
	switch {
	case v.Nil != nil:
		return Value{Kind: "nil"}, nil
	case v.String != nil:
		return Value{Kind: "string", Str: *v.String}, nil
	case v.I4 != nil:
		n, err := strconv.ParseInt(strings.TrimSpace(*v.I4), 10, 64)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: "int", Int: n}, nil
	case v.Int != nil:
		n, err := strconv.ParseInt(strings.TrimSpace(*v.Int), 10, 64)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: "int", Int: n}, nil
	case v.I8 != nil:
		n, err := strconv.ParseInt(strings.TrimSpace(*v.I8), 10, 64)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: "i8", Int: n}, nil
	case v.Boolean != nil:
		s := strings.TrimSpace(*v.Boolean)
		return Value{Kind: "bool", Bool: s == "1" || strings.EqualFold(s, "true")}, nil
	case v.Double != nil:
		f, err := strconv.ParseFloat(strings.TrimSpace(*v.Double), 64)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: "double", Float: f}, nil
	case v.Array != nil:
		out := make([]Value, 0, len(v.Array.Data.Values))
		for _, item := range v.Array.Data.Values {
			parsed, err := item.toValue()
			if err != nil {
				return Value{}, err
			}
			out = append(out, parsed)
		}
		return Value{Kind: "array", Array: out}, nil
	case v.Struct != nil:
		out := make(map[string]Value, len(v.Struct.Members))
		for _, m := range v.Struct.Members {
			parsed, err := m.Value.toValue()
			if err != nil {
				return Value{}, err
			}
			out[m.Name] = parsed
		}
		return Value{Kind: "struct", Struct: out}, nil
	default:
		// Bare text content fallback.
		s := strings.TrimSpace(v.InnerXML)
		if s == "" {
			return Value{Kind: "string", Str: ""}, nil
		}
		return Value{Kind: "string", Str: s}, nil
	}
}
