// Package vision captures the screen for A.R.I.A.
//
// Design:
//   - capture on demand (user request / proactive trigger), never a blind
//     continuous stream: the LLM is the bottleneck, not the capture;
//   - the OS downscales (GPU on macOS) straight to the target size, so a 5K
//     screen never transits through Go at full resolution;
//   - JPEG bytes land in buffers recycled through a sync.Pool; Frame.Release
//     hands them back;
//   - base64 is appended into the caller's buffer (AppendBase64), avoiding
//     the intermediate string of EncodeToString;
//   - Spark's own window is excluded from the capture.
package vision

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// Options tune the capture.
type Options struct {
	// MaxSide caps the longest edge in pixels. 1568 is the usual sweet spot
	// for vision LLMs: larger images are downscaled server-side anyway.
	MaxSide int
	// Quality is the JPEG quality, 1-100.
	Quality int
}

var DefaultOptions = Options{MaxSide: 1568, Quality: 72}

// ErrPermission means the OS denied screen capture (macOS Screen Recording).
var ErrPermission = errors.New("vision: screen recording permission denied")

// Frame is an encoded screenshot ready to be sent to the LLM.
type Frame struct {
	Data      []byte // JPEG bytes; valid until Release.
	Width     int
	Height    int
	Timestamp time.Time
	Took      time.Duration

	buf *[]byte
}

// Release hands the underlying buffer back to the pool. The frame must not
// be used afterwards.
func (f *Frame) Release() {
	if f == nil || f.buf == nil {
		return
	}
	*f.buf = f.Data[:0]
	bufPool.Put(f.buf)
	f.buf, f.Data = nil, nil
}

// AppendBase64 appends the standard base64 encoding of the JPEG to dst.
func (f *Frame) AppendBase64(dst []byte) []byte {
	return base64.StdEncoding.AppendEncode(dst, f.Data)
}

// Capturer grabs the current screen content.
type Capturer interface {
	Capture(ctx context.Context) (*Frame, error)
}

// platform is the OS-specific encoder (capture_<os>.go). It appends the
// JPEG to dst and returns the extended slice.
type platform interface {
	captureJPEG(ctx context.Context, opts Options, dst []byte) (data []byte, w, h int, err error)
}

const initialBufCap = 512 << 10 // a 1568px JPEG is typically 150-400 KB

var bufPool = sync.Pool{New: func() any {
	b := make([]byte, 0, initialBufCap)
	return &b
}}

// Service is the Capturer used by the app. Captures are serialized: the
// native backends reuse their scratch buffers.
type Service struct {
	opts   Options
	native platform
	mu     sync.Mutex
}

func New(opts Options) *Service {
	return &Service{opts: opts, native: newPlatform()}
}

func (s *Service) Capture(ctx context.Context) (*Frame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	buf := bufPool.Get().(*[]byte)
	data, w, h, err := s.native.captureJPEG(ctx, s.opts, (*buf)[:0])
	if err != nil {
		bufPool.Put(buf)
		return nil, err
	}
	return &Frame{Data: data, Width: w, Height: h, Timestamp: start, Took: time.Since(start), buf: buf}, nil
}

// HasPermission reports whether screen capture is currently allowed.
func HasPermission() bool { return hasPermission() }

// RequestPermission triggers the OS permission prompt if needed (macOS
// shows it once; afterwards the user must go through System Settings).
func RequestPermission() { requestPermission() }

// fitSize scales (w, h) down so the longest side is at most maxSide.
func fitSize(w, h, maxSide int) (int, int) {
	if maxSide <= 0 || (w <= maxSide && h <= maxSide) {
		return w, h
	}
	if w >= h {
		return maxSide, max(1, h*maxSide/w)
	}
	return max(1, w*maxSide/h), maxSide
}
