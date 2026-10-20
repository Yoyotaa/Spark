import {useEffect, useRef, useState, type ReactNode} from 'react'
import {keyLabel} from '../../lib/bridge'
import {shortcutLabel, type Shortcut} from '../../lib/settings'

/** A titled card grouping related rows. */
export function Group({title, badge, children}: {title?: string; badge?: ReactNode; children: ReactNode}) {
  return (
    <section className="space-y-2">
      {title && (
        <h3 className="flex items-center gap-2 px-1 font-mono text-[10px] tracking-[0.24em] text-slate-400 uppercase">
          {title}
          {badge}
        </h3>
      )}
      <div className="divide-y divide-white/[0.05] rounded-xl border border-white/[0.06] bg-white/[0.02]">{children}</div>
    </section>
  )
}

/** Label + help on the left, control on the right (or below when wide). */
export function Row({
  label,
  help,
  children,
  stacked,
  disabled,
}: {
  label: string
  help?: ReactNode
  children: ReactNode
  stacked?: boolean
  disabled?: boolean
}) {
  return (
    <div
      className={`flex gap-4 px-4 py-3.5 ${stacked ? 'flex-col' : 'items-center justify-between'} ${
        disabled ? 'pointer-events-none opacity-40' : ''
      }`}
    >
      <div className="min-w-0">
        <div className="text-[13px] font-medium text-slate-100">{label}</div>
        {help && <div className="mt-0.5 text-[11.5px] leading-snug text-slate-400">{help}</div>}
      </div>
      <div className={stacked ? '' : 'shrink-0'}>{children}</div>
    </div>
  )
}

export function Toggle({checked, onChange, label}: {checked: boolean; onChange: (v: boolean) => void; label: string}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className={`relative h-5 w-9 rounded-full border transition-colors duration-200 ${
        checked ? 'border-spark-cyan/60 bg-spark-cyan/35' : 'border-white/10 bg-white/10'
      }`}
    >
      <span
        className={`absolute top-1/2 left-0.5 size-3.5 -translate-y-1/2 rounded-full transition-transform duration-200 ${
          checked ? 'translate-x-4 bg-spark-cyan shadow-[0_0_8px_var(--color-spark-cyan)]' : 'bg-slate-300'
        }`}
      />
    </button>
  )
}

export function Segmented<T extends string | number>({
  value,
  options,
  onChange,
}: {
  value: T
  options: {value: T; label: string}[]
  onChange: (v: T) => void
}) {
  return (
    <div className="inline-flex rounded-lg border border-white/[0.07] bg-black/30 p-0.5">
      {options.map((o) => (
        <button
          key={String(o.value)}
          type="button"
          onClick={() => onChange(o.value)}
          className={`rounded-md px-3 py-1.5 text-[12px] transition-colors ${
            o.value === value
              ? 'bg-spark-cyan/15 text-spark-cyan shadow-[inset_0_0_0_1px_color-mix(in_oklab,var(--color-spark-cyan)_35%,transparent)]'
              : 'text-slate-400 hover:text-slate-200'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

export function Slider({
  value,
  min,
  max,
  step = 1,
  onChange,
  format = (v) => String(v),
  label,
}: {
  value: number
  min: number
  max: number
  step?: number
  onChange: (v: number) => void
  format?: (v: number) => string
  label: string
}) {
  return (
    <div className="flex items-center gap-3">
      <input
        type="range"
        aria-label={label}
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-40 accent-[var(--color-spark-cyan)]"
      />
      <span className="w-14 text-right font-mono text-[11px] text-spark-cyan">{format(value)}</span>
    </div>
  )
}

export function Select({
  value,
  options,
  onChange,
  label,
  className = 'w-64',
}: {
  value: string
  options: {value: string; label: string}[]
  onChange: (v: string) => void
  label: string
  className?: string
}) {
  return (
    <div className={`relative ${className}`}>
      <select
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full appearance-none rounded-lg border border-white/[0.08] bg-black/30 py-2 pr-8 pl-3 text-[12.5px] text-slate-100 outline-none focus:border-spark-cyan/50"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value} className="bg-slate-900">
            {o.label}
          </option>
        ))}
      </select>
      <svg viewBox="0 0 12 12" className="pointer-events-none absolute top-1/2 right-3 size-2.5 -translate-y-1/2 text-slate-400" aria-hidden>
        <path d="M2 4.5l4 4 4-4" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round" />
      </svg>
    </div>
  )
}

export const inputClass =
  'w-full rounded-lg border border-white/[0.08] bg-black/30 px-3 py-2 text-[12.5px] text-slate-100 outline-none transition-colors placeholder:text-slate-500 focus:border-spark-cyan/50'

export function Button({
  children,
  onClick,
  variant = 'default',
  disabled,
  busy,
}: {
  children: ReactNode
  onClick: () => void
  variant?: 'default' | 'primary' | 'danger'
  disabled?: boolean
  busy?: boolean
}) {
  const styles = {
    default: 'border-white/10 bg-white/[0.04] text-slate-200 hover:bg-white/[0.08]',
    primary: 'border-spark-cyan/40 bg-spark-cyan/15 text-spark-cyan hover:bg-spark-cyan/25',
    danger: 'border-rose-400/30 bg-rose-500/10 text-rose-200 hover:bg-rose-500/20',
  }[variant]
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || busy}
      className={`inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-[12px] whitespace-nowrap transition-colors disabled:opacity-40 ${styles}`}
    >
      {busy && <span className="size-3 animate-spin rounded-full border border-current border-t-transparent" />}
      {children}
    </button>
  )
}

export function Badge({children, tone = 'muted'}: {children: ReactNode; tone?: 'muted' | 'ok' | 'warn' | 'soon'}) {
  const styles = {
    muted: 'border-white/10 text-slate-400',
    ok: 'border-emerald-400/30 bg-emerald-400/10 text-emerald-300',
    warn: 'border-amber-400/30 bg-amber-400/10 text-amber-200',
    soon: 'border-spark-violet/40 bg-spark-violet/10 text-spark-violet',
  }[tone]
  return (
    <span className={`rounded-full border px-2 py-0.5 font-mono text-[9px] tracking-[0.14em] normal-case ${styles}`}>
      {children}
    </span>
  )
}

/** Masked secret entry: the stored value is never shown, only a hint. */
export function SecretField({
  placeholder,
  onSave,
}: {
  placeholder: string
  onSave: (value: string) => Promise<void>
}) {
  const [value, setValue] = useState('')
  const [visible, setVisible] = useState(false)
  const [busy, setBusy] = useState(false)

  async function save() {
    if (!value.trim()) return
    setBusy(true)
    try {
      await onSave(value.trim())
      setValue('')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex items-center gap-2">
      <div className="relative flex-1">
        <input
          type={visible ? 'text' : 'password'}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && save()}
          placeholder={placeholder}
          autoComplete="off"
          spellCheck={false}
          className={`${inputClass} pr-9 font-mono`}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          aria-label={visible ? 'Masquer' : 'Afficher'}
          className="absolute top-1/2 right-2 -translate-y-1/2 p-1 text-slate-500 hover:text-slate-200"
        >
          <svg viewBox="0 0 20 20" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden>
            <path d="M1.8 10S5 4.2 10 4.2 18.2 10 18.2 10 15 15.8 10 15.8 1.8 10 1.8 10Z" />
            <circle cx="10" cy="10" r="2.6" />
            {!visible && <path d="M3 17L17 3" />}
          </svg>
        </button>
      </div>
      <Button variant="primary" onClick={save} disabled={!value.trim()} busy={busy}>
        Enregistrer
      </Button>
    </div>
  )
}

export function Kbd({children}: {children: ReactNode}) {
  return (
    <kbd className="rounded-md border border-white/15 bg-white/[0.05] px-2 py-1 font-mono text-[11px] text-slate-100 shadow-[inset_0_-1px_0_rgb(255_255_255/0.08)]">
      {children}
    </kbd>
  )
}

const MODIFIER_CODES = new Set(['AltLeft', 'AltRight', 'ShiftLeft', 'ShiftRight', 'ControlLeft', 'ControlRight', 'MetaLeft', 'MetaRight'])

/** Resolves a keydown to a Shortcut key ("A", "7", "F5", "Space") or "". */
async function keyFromEvent(e: KeyboardEvent): Promise<string> {
  if (e.code === 'Space') return 'Space'
  if (/^F([1-9]|1[0-2])$/.test(e.key)) return e.key
  if (/^Digit[0-9]$/.test(e.code)) return e.code.slice(5)
  // A plain letter is reported as typed (layout-aware). With ⌥ on macOS the
  // browser reports a special character instead: ask Go for the layout.
  if (/^[a-zA-Z]$/.test(e.key)) return e.key.toUpperCase()
  return keyLabel(e.code)
}

/**
 * Click, then press the combination. Escape cancels, Backspace clears.
 * `requireModifier` rejects bare letters, which would block normal typing.
 */
export function ShortcutRecorder({
  value,
  onChange,
  requireModifier = true,
}: {
  value: Shortcut
  onChange: (s: Shortcut) => void
  requireModifier?: boolean
}) {
  const [recording, setRecording] = useState(false)
  const [hint, setHint] = useState<string | null>(null)
  const ref = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!recording) return
    const onKey = async (e: KeyboardEvent) => {
      e.preventDefault()
      e.stopPropagation()
      if (MODIFIER_CODES.has(e.code)) return
      if (e.key === 'Escape') return setRecording(false)
      if (e.key === 'Backspace' || e.key === 'Delete') {
        onChange({key: '', alt: false, shift: false, ctrl: false, cmd: false})
        return setRecording(false)
      }
      const key = await keyFromEvent(e)
      if (!key) return setHint('Touche non prise en charge : lettre, chiffre, F1-F12 ou Espace.')
      const s = {key, alt: e.altKey, shift: e.shiftKey, ctrl: e.ctrlKey, cmd: e.metaKey}
      const hasMod = s.alt || s.shift || s.ctrl || s.cmd
      if (requireModifier && !hasMod && !key.startsWith('F')) {
        return setHint('Ajoute ⌥, ⇧, ⌃ ou ⌘ : une touche seule bloquerait la saisie.')
      }
      setHint(null)
      onChange(s)
      setRecording(false)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [recording, onChange, requireModifier])

  return (
    <div className="flex flex-col items-end gap-1">
      <button
        ref={ref}
        type="button"
        onClick={() => {
          setHint(null)
          setRecording((r) => !r)
        }}
        onBlur={() => setRecording(false)}
        className={`min-w-28 rounded-lg border px-3 py-1.5 text-center font-mono text-[12px] transition-colors ${
          recording
            ? 'animate-pulse border-spark-cyan/60 bg-spark-cyan/10 text-spark-cyan'
            : 'border-white/10 bg-black/30 text-slate-100 hover:border-white/20'
        }`}
      >
        {recording ? 'Appuie…' : shortcutLabel(value)}
      </button>
      {recording && !hint && <span className="text-[10.5px] text-slate-500">Échap : annuler · ⌫ : effacer</span>}
      {hint && <span className="max-w-60 text-right text-[10.5px] text-amber-200/90">{hint}</span>}
    </div>
  )
}
