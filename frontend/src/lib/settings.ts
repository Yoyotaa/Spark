// Mirrors internal/config/settings.go. Plain types (the generated Wails
// classes add constructors we do not need).

export type Provider = 'gemini' | 'claude' | 'openai'
export type Shortcut = {key: string; alt: boolean; shift: boolean; ctrl: boolean; cmd: boolean}

export type Settings = {
  orb?: {x: number; y: number}
  ai: {
    provider: Provider
    models: Record<string, string>
    baseUrls: Record<string, string>
    thinking: 'low' | 'medium' | 'high'
    length: 'short' | 'normal' | 'detailed'
    language: 'fr' | 'en'
    customInstructions: string
    historyTurns: number
    fallback: boolean
  }
  vision: {enabled: boolean; maxSide: number; quality: number; showCursor: boolean}
  proactive: {enabled: boolean; interval: number}
  voice: {
    enabled: boolean
    engine: 'system' | 'gemini'
    neuralVoice: string
    style: string
    voice: string
    rate: number
    readAnswers: boolean
    greeting: boolean
  }
  mic: {
    mode: 'off' | 'ptt' | 'continuous'
    pushToTalk: Shortcut
    sensitivity: number
    wakeWord: boolean
    wakePhrase: string
    cues: boolean
  }
  hotkeys: {toggleChat: Shortcut; toggleVisible: Shortcut; openSettings: Shortcut}
  appearance: {
    accent: Accent
    orbSize: 'small' | 'medium' | 'large'
    panelOpacity: number
    reduceMotion: boolean
    showBubbles: boolean
  }
  advanced: {debug: boolean; pollHz: number}
}

const sc = (key: string, mods: Partial<Shortcut> = {}): Shortcut => ({
  key,
  alt: false,
  shift: false,
  ctrl: false,
  cmd: false,
  ...mods,
})

// Same values as config.Defaults(); used by the browser preview.
export const DEFAULTS: Settings = {
  ai: {
    provider: 'gemini',
    models: {},
    baseUrls: {},
    thinking: 'low',
    length: 'normal',
    language: 'fr',
    customInstructions: '',
    historyTurns: 6,
    fallback: true,
  },
  vision: {enabled: true, maxSide: 1568, quality: 72, showCursor: true},
  proactive: {enabled: false, interval: 60},
  voice: {
    enabled: true,
    engine: 'system',
    neuralVoice: 'Despina',
    style: 'calme, posée et assurée, comme une assistante d’élite',
    voice: '',
    rate: 1,
    readAnswers: true,
    greeting: true,
  },
  mic: {mode: 'ptt', pushToTalk: sc('F8'), sensitivity: 50, wakeWord: true, wakePhrase: 'Aria', cues: true},
  hotkeys: {
    toggleChat: sc('A', {alt: true, shift: true}),
    toggleVisible: sc('H', {alt: true, shift: true}),
    openSettings: sc('P', {alt: true, shift: true}),
  },
  appearance: {accent: 'cyan', orbSize: 'medium', panelOpacity: 82, reduceMotion: false, showBubbles: true},
  advanced: {debug: false, pollHz: 60},
}

export const DEFAULT_MODELS: Record<Provider, string> = {
  gemini: 'gemini-3.6-flash',
  claude: 'claude-opus-5-5',
  openai: '',
}

// Suggestions shown before the provider's list is fetched.
export const MODEL_HINTS: Record<Provider, string[]> = {
  gemini: ['gemini-3.6-flash', 'gemini-3.5-flash-lite', 'gemini-3.8-flash'],
  claude: ['claude-opus-5-5', 'claude-sonnet-5-5', 'claude-haiku-5-5'],
  openai: [],
}

export type Accent = 'cyan' | 'violet' | 'emerald' | 'amber' | 'rose'

export const ACCENTS: Record<Accent, {label: string; light: string; main: string; second: string}> = {
  cyan: {label: 'Cyan', light: '#5eead4', main: '#22d3ee', second: '#8b5cf6'},
  violet: {label: 'Violet', light: '#c4b5fd', main: '#a78bfa', second: '#ec4899'},
  emerald: {label: 'Émeraude', light: '#6ee7b7', main: '#34d399', second: '#22d3ee'},
  amber: {label: 'Ambre', light: '#fde68a', main: '#fbbf24', second: '#f97316'},
  rose: {label: 'Rose', light: '#fda4af', main: '#fb7185', second: '#a78bfa'},
}

export const ORB_SIZES = {small: 48, medium: 60, large: 72} as const

/** Pushes the appearance settings into the CSS theme tokens. */
export function applyAppearance(a: Settings['appearance']) {
  const root = document.documentElement.style
  const c = ACCENTS[a.accent] ?? ACCENTS.cyan
  root.setProperty('--color-spark-teal', c.light)
  root.setProperty('--color-spark-cyan', c.main)
  root.setProperty('--color-spark-violet', c.second)
  root.setProperty('--color-panel', `rgb(8 11 20 / ${a.panelOpacity / 100})`)
  document.body.classList.toggle('reduce-motion', a.reduceMotion)
}

const isMac = typeof navigator !== 'undefined' && /Mac/.test(navigator.platform)

/** Human label, e.g. "⌥⇧A" on macOS, "Alt+Shift+A" on Windows. */
export function shortcutLabel(s: Shortcut): string {
  if (!s.key) return '—'
  const key = s.key === 'Space' ? (isMac ? '␣' : 'Espace') : s.key
  if (isMac) return `${s.ctrl ? '⌃' : ''}${s.alt ? '⌥' : ''}${s.shift ? '⇧' : ''}${s.cmd ? '⌘' : ''}${key}`
  return [s.ctrl && 'Ctrl', s.alt && 'Alt', s.shift && 'Shift', s.cmd && 'Win', key].filter(Boolean).join('+')
}
