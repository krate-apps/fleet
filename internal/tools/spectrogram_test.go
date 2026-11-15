package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSpectrogramRequiresAllowedRoot(t *testing.T) {
	r := &Runner{AllowedRoots: []string{t.TempDir()}}
	_, err := r.Spectrogram(context.Background(), "/etc/passwd", SpectrogramOpts{})
	if err == nil {
		t.Fatal("expected path error")
	}
}

func TestSpectrogramWithSox(t *testing.T) {
	if _, err := exec.LookPath("sox"); err != nil {
		t.Skip("sox not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "tone.wav")
	cmd := exec.Command("sox", "-n", src, "synth", "1", "sine", "440")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare wav: %v (%s)", err, out)
	}
	r := &Runner{AllowedRoots: []string{dir}, SoxPath: "sox"}
	img, err := r.Spectrogram(context.Background(), src, SpectrogramOpts{Title: "tone", Width: 400})
	if err != nil {
		t.Fatal(err)
	}
	if img == nil || img.DataURL == "" || img.Bytes == 0 {
		t.Fatalf("%#v", img)
	}
	_ = os.Remove(src)
}
