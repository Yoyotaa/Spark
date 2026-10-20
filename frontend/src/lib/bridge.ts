// Thin wrapper over the generated Wails bindings so the UI also runs in a
// plain browser (`npm run dev`) where window.go does not exist.
import * as App from '../../wailsjs/go/app/App'
import type {config, overlay} from '../../wailsjs/go/models'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const inWails = typeof window !== 'undefined' && 'go' in window

// Mirrors the constants of internal/app/app.go.
const EventToggleChat = 'aria:toggle-chat'
const EventVisibility = 'spark:visibility'
const EventSay = 'aria:say'
const EventSpeaking = 'aria:speaking'

export type Region = Pick<overlay.Rect, 'x' | 'y' | 'w' | 'h'>
export type Point = {x: number; y: number}

// In a plain browser, events go through a local bus so the UI stays demoable.
const localBus = new EventTarget()

function emitLocal(event: string, ...args: unknown[]) {
  localBus.dispatchEvent(new CustomEvent(event, {detail: args}))
}

function on<T extends unknown[]>(event: string, cb: (...args: T) => void): () => void {
  if (inWails) return EventsOn(event, cb as (...data: unknown[]) => void)
  const listener = (e: Event) => cb(...((e as CustomEvent).detail as T))
  localBus.addEventListener(event, listener)
  return () => localBus.removeEventListener(event, listener)
}

export function setInteractiveRegions(regions: Region[]): void {
  if (inWails) void App.SetInteractiveRegions(regions as overlay.Rect[])
}

export function setMouseCaptured(v: boolean): void {
  if (inWails) void App.SetMouseCaptured(v)
}

export function setChatOpen(open: boolean): void {
  if (inWails) void App.SetChatOpen(open)
}

/** Starts the welcome sequence (voice + bubble). */
let booted = false

export function boot(): void {
  if (booted) return // StrictMode mounts twice in dev
  booted = true
  if (inWails) {
    void App.Boot()
    return
  }
  // Browser preview: fake the greeting (no voice).
  setTimeout(() => {
    emitLocal(EventSay, 'Bonsoir. A.R.I.A en ligne, je suis prête.')
    emitLocal(EventSpeaking, true)
    setTimeout(() => emitLocal(EventSpeaking, false), 2600)
  }, 600)
}

export async function getSettings(): Promise<Partial<config.Settings>> {
  return inWails ? App.GetSettings() : {}
}

export function saveOrbPosition(p: Point): void {
  if (inWails) void App.SetOrbPosition(p.x, p.y)
}

export async function isDebug(): Promise<boolean> {
  return inWails ? App.Debug() : new URLSearchParams(location.search).has('debug')
}

/** Global hotkey ⌥⇧A. */
export const onToggleChat = (cb: () => void) => on(EventToggleChat, cb)
/** Global hotkey ⌥⇧H. */
export const onVisibility = (cb: (visible: boolean) => void) => on(EventVisibility, cb)
/** A.R.I.A says something (greeting, answers). */
export const onSay = (cb: (text: string) => void) => on(EventSay, cb)
/** A.R.I.A's voice starts or stops. */
export const onSpeaking = (cb: (speaking: boolean) => void) => on(EventSpeaking, cb)
