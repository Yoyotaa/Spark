//go:build windows

package audio

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows: waveIn capture (16 kHz mono PCM, no cgo), utterances transcribed
// by a cloud Transcriber (Gemini).

const (
	sampleRate    = 16000
	bufferMillis  = 50
	bufferCount   = 8
	waveMapper    = ^uintptr(0) // WAVE_MAPPER (UINT -1)
	callbackNull  = 0
	whdrDone      = 0x1
	maxUtterance  = 30 * time.Second
	transcribeMax = 20 * time.Second
)

var (
	winmm               = windows.NewLazySystemDLL("winmm.dll")
	procWaveInOpen      = winmm.NewProc("waveInOpen")
	procWaveInPrepare   = winmm.NewProc("waveInPrepareHeader")
	procWaveInUnprepare = winmm.NewProc("waveInUnprepareHeader")
	procWaveInAddBuffer = winmm.NewProc("waveInAddBuffer")
	procWaveInStart     = winmm.NewProc("waveInStart")
	procWaveInStop      = winmm.NewProc("waveInStop")
	procWaveInReset     = winmm.NewProc("waveInReset")
	procWaveInClose     = winmm.NewProc("waveInClose")
	procMessageBeep     = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBeep")
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

type windowsInput struct {
	ev    InputEvents
	cloud Transcriber

	mu        sync.Mutex
	handle    uintptr
	headers   []*waveHdr
	buffers   [][]byte // referenced so the GC keeps them (the Go heap does not move)
	stop      chan struct{}
	stopped   chan struct{}
	recording bool
	pcm       []byte
	recent    []byte // last ~0.5 s, prepended to an utterance (pre-roll)
	lang      string
	started   time.Time
}

func NewInput(ev InputEvents, cloud Transcriber) Input {
	return &windowsInput{ev: ev, cloud: cloud}
}

func (w *windowsInput) Live() bool { return false }

func (w *windowsInput) StartMic() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.handle != 0 {
		return nil
	}
	f := waveFormatEx{FormatTag: 1, Channels: 1, SamplesPerSec: sampleRate, AvgBytesPerSec: sampleRate * 2, BlockAlign: 2, BitsPerSample: 16}
	var h uintptr
	if r, _, _ := procWaveInOpen.Call(uintptr(unsafe.Pointer(&h)), waveMapper, uintptr(unsafe.Pointer(&f)), 0, 0, callbackNull); r != 0 {
		return errors.New("aucun micro disponible")
	}
	w.handle = h
	size := sampleRate * 2 * bufferMillis / 1000
	w.headers, w.buffers = nil, nil
	for range bufferCount {
		buf := make([]byte, size)
		hdr := &waveHdr{Data: uintptr(unsafe.Pointer(&buf[0])), BufferLength: uint32(size)}
		procWaveInPrepare.Call(h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		procWaveInAddBuffer.Call(h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		w.headers = append(w.headers, hdr)
		w.buffers = append(w.buffers, buf)
	}
	procWaveInStart.Call(h)
	w.stop, w.stopped = make(chan struct{}), make(chan struct{})
	go w.pump(h, w.stop, w.stopped)
	return nil
}

// pump polls the buffers: no callback thread to marshal into Go.
func (w *windowsInput) pump(h uintptr, stop, stopped chan struct{}) {
	defer close(stopped)
	t := time.NewTicker(bufferMillis / 2 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		for i, hdr := range w.headers {
			if hdr.Flags&whdrDone == 0 {
				continue
			}
			data := w.buffers[i][:hdr.BytesRecorded]
			w.consume(data)
			hdr.Flags &^= whdrDone
			hdr.BytesRecorded = 0
			procWaveInAddBuffer.Call(h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
		}
	}
}

func (w *windowsInput) consume(data []byte) {
	var sum float64
	n := len(data) / 2
	for i := 0; i+1 < len(data); i += 2 {
		s := float64(int16(uint16(data[i])|uint16(data[i+1])<<8)) / 32768
		sum += s * s
	}
	if n > 0 && w.ev.Level != nil {
		w.ev.Level(float32(math.Sqrt(sum / float64(n))))
	}
	w.mu.Lock()
	if !w.recording {
		const prerollBytes = sampleRate * 2 / 2
		w.recent = append(w.recent, data...)
		if over := len(w.recent) - prerollBytes; over > 0 {
			w.recent = append(w.recent[:0], w.recent[over:]...)
		}
	}
	if w.recording {
		w.pcm = append(w.pcm, data...)
		if time.Since(w.started) > maxUtterance {
			w.mu.Unlock()
			w.End()
			return
		}
	}
	w.mu.Unlock()
}

func (w *windowsInput) StopMic() {
	w.mu.Lock()
	h, stop, stopped := w.handle, w.stop, w.stopped
	w.handle = 0
	w.mu.Unlock()
	if h == 0 {
		return
	}
	close(stop)
	<-stopped
	procWaveInStop.Call(h)
	procWaveInReset.Call(h)
	for _, hdr := range w.headers {
		procWaveInUnprepare.Call(h, uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr))
	}
	procWaveInClose.Call(h)
}

func (w *windowsInput) Begin(lang string) error {
	if w.cloud == nil {
		return errors.New("sous Windows, la reconnaissance vocale passe par Gemini : ajoute une clé Gemini dans Paramètres › Intelligence")
	}
	w.mu.Lock()
	w.recording, w.lang, w.started = true, lang, time.Now()
	w.pcm = append(w.pcm[:0], w.recent...)
	w.recent = w.recent[:0]
	w.mu.Unlock()
	return nil
}

func (w *windowsInput) End() {
	w.mu.Lock()
	if !w.recording {
		w.mu.Unlock()
		return
	}
	w.recording = false
	pcm := append([]byte(nil), w.pcm...)
	lang := w.lang
	w.mu.Unlock()

	go func() {
		// Less than ~0.3 s is a mis-press, not speech.
		if len(pcm) < sampleRate*2*3/10 {
			w.final("")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), transcribeMax)
		defer cancel()
		text, err := w.cloud.Transcribe(ctx, wav(pcm, sampleRate), lang)
		if err != nil {
			if w.ev.Error != nil {
				w.ev.Error(err)
			}
			return
		}
		w.final(text)
	}()
}

func (w *windowsInput) final(text string) {
	if w.ev.Final != nil {
		w.ev.Final(text)
	}
}

func (w *windowsInput) Cancel() {
	w.mu.Lock()
	w.recording = false
	w.mu.Unlock()
}

// Windows asks for the microphone itself (privacy settings) on first use.
func MicPermission() Permission    { return PermissionGranted }
func SpeechPermission() Permission { return PermissionGranted }

func RequestPermissions(done func(ok bool)) { done(true) }

// PlayCue plays a short system sound when listening starts or stops.
func PlayCue(start bool) {
	const mbOK, mbIconAsterisk = 0x0, 0x40
	if start {
		procMessageBeep.Call(mbIconAsterisk)
	} else {
		procMessageBeep.Call(mbOK)
	}
}
