package vision

import (
	"context"
	"encoding/base64"
	"testing"
)

type fakePlatform struct{ payload []byte }

func (f fakePlatform) captureJPEG(_ context.Context, _ Options, dst []byte) ([]byte, int, int, error) {
	return append(dst, f.payload...), 640, 360, nil
}

func TestCaptureReleaseAndBase64(t *testing.T) {
	s := &Service{opts: DefaultOptions, native: fakePlatform{payload: []byte("jpeg-bytes")}}
	f, err := s.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "jpeg-bytes" || f.Width != 640 || f.Height != 360 {
		t.Fatalf("unexpected frame %+v", f)
	}
	got := f.AppendBase64([]byte("data:image/jpeg;base64,"))
	want := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("jpeg-bytes"))
	if string(got) != want {
		t.Fatalf("base64 = %q", got)
	}
	f.Release()
	f.Release() // idempotent
	if f.Data != nil {
		t.Fatal("Release must drop the data")
	}
}

func TestFitSize(t *testing.T) {
	for _, c := range []struct{ w, h, max, ww, wh int }{
		{5120, 2880, 1568, 1568, 882},
		{1080, 1920, 1568, 882, 1568},
		{800, 600, 1568, 800, 600},
	} {
		if w, h := fitSize(c.w, c.h, c.max); w != c.ww || h != c.wh {
			t.Errorf("fitSize(%d,%d,%d) = %d,%d want %d,%d", c.w, c.h, c.max, w, h, c.ww, c.wh)
		}
	}
}

func BenchmarkCaptureFake(b *testing.B) {
	s := &Service{opts: DefaultOptions, native: fakePlatform{payload: make([]byte, 300<<10)}}
	b.ReportAllocs()
	for b.Loop() {
		f, _ := s.Capture(context.Background())
		f.Release()
	}
}
