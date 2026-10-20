// Thin wrapper over the generated Wails bindings so the UI also runs in a
// plain browser (`npm run dev`) where window.go does not exist.
import * as App from '../../wailsjs/go/app/App'
import type {overlay} from '../../wailsjs/go/models'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const inWails = typeof window !== 'undefined' && 'go' in window

// Mirrors the constants of internal/app/app.go.
const EventToggleChat = 'aria:toggle-chat'
const EventVisibility = 'spark:visibility'

export type Region = Pick<overlay.Rect, 'x' | 'y' | 'w' | 'h'>

export function setInteractiveRegions(regions: Region[]): void {
  if (inWails) void App.SetInteractiveRegions(regions as overlay.Rect[])
}

export function setMouseCaptured(v: boolean): void {
  if (inWails) void App.SetMouseCaptured(v)
}

export function setChatOpen(open: boolean): void {
  if (inWails) void App.SetChatOpen(open)
}

export async function isDebug(): Promise<boolean> {
  return inWails ? App.Debug() : true
}

/** Global hotkey ⌥⇧A. Returns an unsubscribe function. */
export function onToggleChat(cb: () => void): () => void {
  return inWails ? EventsOn(EventToggleChat, cb) : () => {}
}

/** Global hotkey ⌥⇧H. Returns an unsubscribe function. */
export function onVisibility(cb: (visible: boolean) => void): () => void {
  return inWails ? EventsOn(EventVisibility, cb) : () => {}
}
