// Thin wrapper over the generated Wails bindings so the UI also runs in a
// plain browser (`npm run dev`) where window.go does not exist.
import * as App from '../../wailsjs/go/app/App'
import type {app, config, overlay} from '../../wailsjs/go/models'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {DEFAULTS, type Provider, type Settings} from './settings'

const inWails = typeof window !== 'undefined' && 'go' in window

// Mirrors the constants of internal/app/app.go.
const EventToggleChat = 'aria:toggle-chat'
const EventVisibility = 'spark:visibility'
const EventOpenSettings = 'spark:open-settings'
const EventSay = 'aria:say'
const EventSpeaking = 'aria:speaking'
const EventDelta = 'aria:delta'
const EventDone = 'aria:done'

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

/** Takes the keyboard (chat or settings in use) or hands it back. */
export function setKeyboardFocus(focused: boolean): void {
  if (inWails) void App.SetKeyboardFocus(focused)
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

// ---- Settings -------------------------------------------------------------

let previewSettings: Settings = structuredClone(DEFAULTS)

export async function getSettings(): Promise<Settings> {
  return inWails ? ((await App.GetSettings()) as unknown as Settings) : structuredClone(previewSettings)
}

export type SaveResult = {settings: Settings; hotkeyError?: string}

/** Persists and applies; resolves with the normalized settings. */
export async function saveSettings(s: Settings): Promise<SaveResult> {
  if (inWails) return (await App.SaveSettings(s as unknown as config.Settings)) as unknown as SaveResult
  previewSettings = structuredClone(s)
  return {settings: structuredClone(s)}
}

export async function resetSettings(): Promise<SaveResult> {
  if (inWails) return (await App.ResetSettings()) as unknown as SaveResult
  previewSettings = {...structuredClone(DEFAULTS), orb: previewSettings.orb}
  return {settings: structuredClone(previewSettings)}
}

export type KeyInfo = {source: '' | 'keychain' | 'env'; hint: string; envNames: string[]}
export type AIStatus = {ready: boolean; name: string; error?: string; keys: Record<string, KeyInfo>}

const previewKeys: Record<string, KeyInfo> = Object.fromEntries(
  (['gemini', 'claude', 'openai'] as Provider[]).map((p) => [p, {source: '', hint: '', envNames: []}]),
)

export async function getAIStatus(): Promise<AIStatus> {
  if (inWails) return (await App.GetAIStatus()) as AIStatus
  return {ready: false, name: 'aperçu navigateur', error: 'Aperçu : pas de connexion IA', keys: previewKeys}
}

/** Stores the key in the OS keychain ("" deletes it). */
export async function setAPIKey(provider: Provider, key: string): Promise<AIStatus> {
  if (inWails) return (await App.SetAPIKey(provider, key)) as AIStatus
  previewKeys[provider] = key ? {source: 'keychain', hint: key.slice(-4), envNames: []} : {source: '', hint: '', envNames: []}
  return getAIStatus()
}

export async function listModels(provider: Provider): Promise<string[]> {
  if (inWails) return App.ListModels(provider)
  throw new Error('Indisponible dans l’aperçu navigateur')
}

export type TestResult = {millis: number; reply: string; model: string}

export async function testAI(provider: Provider): Promise<TestResult> {
  if (inWails) return App.TestAI(provider)
  throw new Error('Indisponible dans l’aperçu navigateur')
}

export type Voice = {name: string; lang: string}

export async function listVoices(): Promise<Voice[]> {
  if (inWails) return App.ListVoices()
  return [
    {name: 'Amélie', lang: 'fr_CA'},
    {name: 'Audrey (Premium)', lang: 'fr_FR'},
    {name: 'Samantha', lang: 'en_US'},
  ]
}

export function testVoice(): void {
  if (inWails) void App.TestVoice()
}

export function clearConversation(): void {
  if (inWails) void App.ClearConversation()
}

/** Letter typed by a physical key (KeyboardEvent.code) on the current layout. */
export async function keyLabel(code: string): Promise<string> {
  if (inWails) return App.KeyLabel(code)
  return /^Key[A-Z]$/.test(code) ? code.slice(3) : ''
}

export type AppInfo = {version: string; platform: string; settingsPath: string}

export async function getAppInfo(): Promise<AppInfo> {
  if (inWails) return App.GetAppInfo()
  return {version: 'dev', platform: 'navigateur', settingsPath: '(aperçu : en mémoire)'}
}

export function saveOrbPosition(p: Point): void {
  if (inWails) void App.SetOrbPosition(p.x, p.y)
}

export type Capture = Pick<app.Capture, 'width' | 'height' | 'bytes' | 'millis' | 'dataUrl'>
export type Turn = {id: number; capture?: Capture}
export type Delta = {id: number; text: string}
export type Done = {id: number; error?: string; interrupted?: boolean}

let fakeTurn = 0

/**
 * Asks A.R.I.A; resolves once the request is accepted, the answer streams
 * through onDelta/onDone. Rejects with a user-facing message.
 */
export async function ask(text: string, withScreen: boolean): Promise<Turn> {
  if (inWails) return App.Ask(text, withScreen) as Promise<Turn>
  // Browser preview: fake a streamed answer.
  const id = ++fakeTurn
  const words = 'Aperçu navigateur : je ne vois pas ton écran ici, lance Spark avec wails dev.'.split(' ')
  words.forEach((w, i) => setTimeout(() => emitLocal(EventDelta, {id, text: (i ? ' ' : '') + w}), 400 + i * 70))
  setTimeout(() => emitLocal(EventDone, {id}), 450 + words.length * 70)
  return {id}
}


export async function isDebug(): Promise<boolean> {
  return inWails ? App.Debug() : new URLSearchParams(location.search).has('debug')
}

/** Global hotkey ⌥⇧A. */
export const onToggleChat = (cb: () => void) => on(EventToggleChat, cb)
/** Global hotkey: open the settings. */
export const onOpenSettings = (cb: () => void) => on(EventOpenSettings, cb)
/** Global hotkey ⌥⇧H. */
export const onVisibility = (cb: (visible: boolean) => void) => on(EventVisibility, cb)
/** A.R.I.A says something (greeting, answers). */
export const onSay = (cb: (text: string) => void) => on(EventSay, cb)
/** Streamed answer text. */
export const onDelta = (cb: (d: Delta) => void) => on(EventDelta, cb)
/** End of an answer. */
export const onDone = (cb: (d: Done) => void) => on(EventDone, cb)
/** A.R.I.A's voice starts or stops. */
export const onSpeaking = (cb: (speaking: boolean) => void) => on(EventSpeaking, cb)
