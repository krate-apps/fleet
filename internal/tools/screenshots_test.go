package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScreenshotsRequiresAllowedRoot(t *testing.T) {
	r := &Runner{AllowedRoots: []string{t.TempDir()}}
	_, err := r.Screenshots(context.Background(), "/etc/passwd", ScreenshotOpts{Count: 1})
	if err == nil {
		t.Fatal("expected path error")
	}
}

func TestScreenshotsWithFfmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.mp4")
	// 2s solid color test pattern.
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=blue:s=320x240:d=2",
		"-y", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare clip: %v (%s)", err, out)
	}
	r := &Runner{AllowedRoots: []string{dir}, FfmpegPath: "ffmpeg"}
	frames, err := r.Screenshots(context.Background(), src, ScreenshotOpts{
		Count: 2, Offset: 0, Interval: 1, Width: 160,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) == 0 {
		t.Fatal("no frames")
	}
	if frames[0].DataURL == "" || frames[0].Bytes == 0 {
		t.Fatalf("%#v", frames[0])
	}
	_ = os.Remove(src)
}
