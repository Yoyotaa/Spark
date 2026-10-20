//go:build windows

package audio

import (
	"context"
	"errors"
	"sync"
	"time"
	"unsafe"
)

// waveOut streaming: each written chunk becomes a buffer queued to the
// device; Drain polls until all are done.

var (
	procWaveOutOpen      = winmm.NewProc("waveOutOpen")
	procWaveOutPrepare   = winmm.NewProc("waveOutPrepareHeader")
	procWaveOutUnprepare = winmm.NewProc("waveOutUnprepareHeader")
	procWaveOutWrite     = winmm.NewProc("waveOutWrite")
	procWaveOutReset     = winmm.NewProc("waveOutReset")
	procWaveOutClose     = winmm.NewProc("waveOutClose")
)

type windowsPlayer struct {
	mu      sync.Mutex
	handle  uintptr
	queued  []*waveHdr
	buffers [][]byte
}

func newPlayer() Player { return &windowsPlayer{} }

func (p *windowsPlayer) Start(rate int) error {
	p.Stop()
	p.mu.Lock()
	defer p.mu.Unlock()
	f := waveFormatEx{FormatTag: 1, Channels: 1, SamplesPerSec: uint32(rate), AvgBytesPerSec: uint32(rate * 2), BlockAlign: 2, BitsPerSample: 16}
	var h uintptr
	if r, _, _ := procWaveOutOpen.Call(uintptr(unsafe.Pointer(&h)), waveMapper, uintptr(unsafe.Pointer(&f)), 0, 0, callbackNull); r != 0 {
		return errors.New("sortie audio indisponible")
	}
	p.handle = h
	return nil
}

func (p *windowsPlayer) Write(pcm []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 || len(pcm) == 0 {
		return
	}
	buf := append([]byte(nil), pcm...)
	hdr := &waveHdr{Data: uintptr(unsafe.Pointer(&buf[0])), BufferLength: uint32(len(buf))}
	procWaveOutPrepare.Call(p.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
	procWaveOutWrite.Call(p.handle, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
	p.queued = append(p.queued, hdr)
	p.buffers = append(p.buffers, buf)
}

func (p *windowsPlayer) Drain(ctx context.Context) error {
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for {
		p.mu.Lock()
		all := true
		for _, h := range p.queued {
			if h.Flags&whdrDone == 0 {
				all = false
				break
			}
		}
		p.mu.Unlock()
		if all {
			return nil
		}
		select {
		case <-ctx.Done():
			p.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (p *windowsPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return
	}
	procWaveOutReset.Call(p.handle)
	for _, h := range p.queued {
		procWaveOutUnprepare.Call(p.handle, uintptr(unsafe.Pointer(h)), unsafe.Sizeof(*h))
	}
	procWaveOutClose.Call(p.handle)
	p.handle, p.queued, p.buffers = 0, nil, nil
}
