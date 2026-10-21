package app

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"spark/internal/ai"
	"spark/internal/vision"
)

// The proactive mode: A.R.I.A looks at the screen every Interval seconds
// and speaks only when something matters. It stays out of the way: no look
// while the user types, talks or while A.R.I.A is already answering, and
// none when the screen did not change.

const proactiveTimeout = 30 * time.Second

type proactive struct {
	lastSize atomic.Int64 // JPEG size of the previous look
	lastLook atomic.Int64 // unix nanos
	focused  atomic.Bool  // chat or settings in use
}

func (a *App) proactiveLoop(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		cfg := a.current()
		if !cfg.Proactive.Enabled {
			continue
		}
		interval := time.Duration(cfg.Proactive.Interval) * time.Second
		if time.Since(time.Unix(0, a.pro.lastLook.Load())) < interval || !a.proactiveFree() {
			continue
		}
		a.pro.lastLook.Store(time.Now().UnixNano())
		a.lookOnce(ctx)
	}
}

// proactiveFree reports whether A.R.I.A may speak up now.
func (a *App) proactiveFree() bool {
	if !a.visible.Load() || a.pro.focused.Load() || a.voice.speaking.Load() {
		return false
	}
	a.voice.mu.Lock()
	listening := a.voice.active
	a.voice.mu.Unlock()
	if listening {
		return false
	}
	if !a.askMu.TryLock() {
		return false
	}
	defer a.askMu.Unlock()
	return a.askCancel == nil
}

// screenChanged compares JPEG sizes: a cheap but effective signal (a
// static screen compresses to the same size).
func (a *App) screenChanged(f *vision.Frame) bool {
	size := int64(len(f.Data))
	prev := a.pro.lastSize.Swap(size)
	if prev == 0 {
		return true
	}
	diff := float64(size-prev) / float64(prev)
	return diff > 0.01 || diff < -0.01
}

func (a *App) lookOnce(parent context.Context) {
	a.mu.RLock()
	llm, cfg := a.llm, a.cfg
	a.mu.RUnlock()
	if llm == nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, proactiveTimeout)
	defer cancel()

	frame, _, err := a.capture(ctx)
	if err != nil {
		a.debugf("proactive: capture: %v", err)
		return
	}
	defer frame.Release()
	if !a.screenChanged(frame) {
		a.debugf("proactive: screen unchanged, skipped")
		return
	}

	system := ai.SystemPrompt(ai.PromptOptions{Language: cfg.AI.Language, Length: "short", Custom: cfg.AI.CustomInstructions}) +
		ai.ProactivePrompt(cfg.AI.Language)
	look := map[string]string{"fr": "Coup d'œil.", "en": "Quick look."}[cfg.AI.Language]
	stream, err := llm.Stream(ctx, system, []ai.Message{{Role: ai.RoleUser, Text: look, Image: frame.Data}})
	if err != nil {
		a.debugf("proactive: %v", err)
		return
	}
	var b strings.Builder
	for c := range stream {
		if c.Err != nil {
			a.debugf("proactive: %v", c.Err)
			return
		}
		b.WriteString(c.Text)
	}
	remark := strings.TrimSpace(b.String())
	if isSilence(remark) || !a.proactiveFree() {
		a.debugf("proactive: nothing to say")
		return
	}
	a.debugf("proactive: %q", remark)

	// Keep it in the conversation so a follow-up question has context.
	a.convMu.Lock()
	a.history = append(a.history,
		ai.Message{Role: ai.RoleUser, Text: look + " [capture d'écran jointe]"},
		ai.Message{Role: ai.RoleModel, Text: remark})
	a.convMu.Unlock()
	a.say(remark)
}

func isSilence(s string) bool {
	s = strings.Trim(strings.ToUpper(strings.TrimSpace(s)), ".!« »\"")
	return s == "" || s == ai.Silence || strings.HasPrefix(s, ai.Silence+" ")
}
