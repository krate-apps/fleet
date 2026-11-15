package rtorrent

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeCall(t *testing.T) {
	body, err := encodeCall("system.client_version", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "<methodName>system.client_version</methodName>") {
		t.Fatalf("body=%s", s)
	}
}

func TestDecodeStringResponse(t *testing.T) {
	xml := `<?xml version="1.0"?><methodResponse><params><param><value><string>0.9.8</string></value></param></params></methodResponse>`
	v, err := decodeResponse(bytes.NewReader([]byte(xml)))
	if err != nil {
		t.Fatal(err)
	}
	if v.AsString() != "0.9.8" {
		t.Fatalf("got %q", v.AsString())
	}
}

func TestDecodeFault(t *testing.T) {
	xml := `<?xml version="1.0"?><methodResponse><fault><value><struct>
<member><name>faultCode</name><value><i4>-500</i4></value></member>
<member><name>faultString</name><value><string>Unsupported</string></value></member>
</struct></value></fault></methodResponse>`
	_, err := decodeResponse(bytes.NewReader([]byte(xml)))
	if err == nil || !strings.Contains(err.Error(), "Unsupported") {
		t.Fatalf("err=%v", err)
	}
}

func TestDecodeMulticallArray(t *testing.T) {
	// Columns follow torrentFields order in torrents.go.
	xml := `<?xml version="1.0"?><methodResponse><params><param><value><array><data>
<value><array><data>
<value><string>ABC</string></value>
<value><string>demo.iso</string></value>
<value><i8>1000</i8></value>
<value><i8>500</i8></value>
<value><i8>500</i8></value>
<value><i4>0</i4></value>
<value><i4>1</i4></value>
<value><i4>1</i4></value>
<value><i4>0</i4></value>
<value><i8>10</i8></value>
<value><i8>20</i8></value>
<value><i8>500</i8></value>
<value><i8>200</i8></value>
<value><i8>1500</i8></value>
<value><i4>2</i4></value>
<value><string>/data</string></value>
<value><string>movies</string></value>
<value><i8>1700000000</i8></value>
<value><i8>0</i8></value>
</data></array></value>
</data></array></value></param></params></methodResponse>`
	v, err := decodeResponse(bytes.NewReader([]byte(xml)))
	if err != nil {
		t.Fatal(err)
	}
	torrents := make([]Torrent, 0)
	for _, row := range v.Array {
		tr, err := parseTorrentRow(row)
		if err != nil {
			t.Fatal(err)
		}
		torrents = append(torrents, tr)
	}
	if len(torrents) != 1 {
		t.Fatalf("len=%d", len(torrents))
	}
	tr := torrents[0]
	if tr.Hash != "ABC" || tr.Name != "demo.iso" || tr.State != "downloading" {
		t.Fatalf("%+v", tr)
	}
	if tr.Ratio != 1.5 || tr.Priority != 2 || tr.Label != "movies" {
		t.Fatalf("%+v", tr)
	}
	if tr.ETASeconds != 50 { // left 500 / rate 10
		t.Fatalf("eta=%d", tr.ETASeconds)
	}
}

func TestBuildSCGIRequest(t *testing.T) {
	body := []byte("hello")
	payload := buildSCGIRequest(body)
	if !bytes.Contains(payload, []byte("CONTENT_LENGTH\x005\x00SCGI\x001\x00REQUEST_METHOD\x00POST\x00")) {
		t.Fatalf("payload=%q", payload)
	}
	if !bytes.HasSuffix(payload, body) {
		t.Fatalf("missing body")
	}
}
