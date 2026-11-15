package fsops

import "testing"

func TestDecodeTextBytesCP437(t *testing.T) {
	raw := []byte{0xB3} // box-drawing vertical in CP437
	text, enc := DecodeTextBytes("demo.nfo", raw, "")
	if enc != "cp437" {
		t.Fatalf("enc=%s", enc)
	}
	if text != string(rune(0x2502)) {
		t.Fatalf("got %q", text)
	}
}

func TestDecodeTextBytesForceUTF8(t *testing.T) {
	raw := []byte{0xB3}
	text, enc := DecodeTextBytes("demo.nfo", raw, "utf8")
	if enc != "utf8" || text != string(raw) {
		t.Fatalf("%q %s", text, enc)
	}
}

func TestDecodeTextBytesForceCP437(t *testing.T) {
	raw := []byte("hi")
	_, enc := DecodeTextBytes("x.txt", raw, "cp437")
	if enc != "cp437" {
		t.Fatalf("enc=%s", enc)
	}
}

func TestDecodeTextBytesUTF8NFO(t *testing.T) {
	raw := []byte("hello nfo")
	text, enc := DecodeTextBytes("x.nfo", raw, "")
	if enc != "utf8" || text != "hello nfo" {
		t.Fatalf("%q %s", text, enc)
	}
}
