// Package vision captures the screen for A.R.I.A.
//
// Planned design (phase 2):
//   - capture on demand (user request / proactive trigger), never a blind
//     continuous stream: the LLM is the bottleneck, not the capture;
//   - one capture goroutine, frames downscaled (e.g. max 1280px) and JPEG
//     encoded into buffers recycled through a sync.Pool;
//   - base64 encoded straight into a pooled buffer (base64.StdEncoding.Encode)
//     to avoid the intermediate string copy of EncodeToString;
//   - the overlay window is excluded from capture (NSWindowSharingNone /
//     WDA_EXCLUDEFROMCAPTURE, see package overlay).
package vision

import (
	"context"
	"time"
)

// Frame is an encoded screenshot ready to be sent to the LLM.
type Frame struct {
	Data      []byte // JPEG bytes; owned by the Capturer until Release.
	Width     int
	Height    int
	Timestamp time.Time

	release func()
}

// Release hands the underlying buffer back to the pool.
func (f *Frame) Release() {
	if f.release != nil {
		f.release()
		f.release = nil
	}
}

// Capturer grabs the current screen content.
type Capturer interface {
	Capture(ctx context.Context) (*Frame, error)
}
