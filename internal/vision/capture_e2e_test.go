package vision

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// Real capture, opt-in: needs a display and the Screen Recording permission.
//
//	SPARK_VISION_E2E=1 go test -run E2E -v ./internal/vision
func TestCaptureE2E(t *testing.T) {
	if os.Getenv("SPARK_VISION_E2E") != "1" {
		t.Skip("set SPARK_VISION_E2E=1")
	}
	s := New(DefaultOptions)
	for i := range 3 { // the first call pays the framework warm-up
		f, err := s.Capture(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(f.Data))
		if err != nil {
			t.Fatalf("not a JPEG: %v", err)
		}
		if cfg.Width != f.Width || cfg.Height != f.Height || max(f.Width, f.Height) > DefaultOptions.MaxSide {
			t.Fatalf("size mismatch: frame %dx%d, jpeg %dx%d", f.Width, f.Height, cfg.Width, cfg.Height)
		}
		t.Logf("capture %d: %dx%d, %d KB, %v", i, f.Width, f.Height, len(f.Data)>>10, f.Took)
		if out := os.Getenv("SPARK_VISION_OUT"); out != "" && i == 2 {
			_ = os.WriteFile(filepath.Join(out, "capture.jpg"), f.Data, 0o644)
		}
		f.Release()
	}
}
