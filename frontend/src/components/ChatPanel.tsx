import {useEffect, useRef, useState, type FormEvent} from 'react'
import type {Capture} from '../lib/bridge'
import {HudCorners} from './HudCorners'
import {SparkLogo} from './SparkLogo'
import {Waveform} from './Waveform'

export type Message = {
  id: string
  from: 'user' | 'aria'
  text: string
  at: Date
  image?: Capture
  streaming?: boolean
  error?: boolean
  voice?: boolean // spoken question
}

type Props = {
  open: boolean
  speaking: boolean
  waiting: boolean
  aiReady: boolean
  seeScreen: boolean
  listening: 'idle' | 'listening' | 'transcribing'
  partial: string
  messages: Message[]
  onSend: (text: string) => void
  onToggleScreen: () => void
  onOpenSettings: () => void
  onStop: () => void
  onClose: () => void
  toggleLabel: string
}

const time = (d: Date) => d.toLocaleTimeString('fr-FR', {hour: '2-digit', minute: '2-digit'})

function Module({name, on}: {name: string; on: boolean}) {
  return (
    <span className={`flex items-center gap-1.5 ${on ? 'text-spark-cyan/80' : 'text-slate-600'}`}>
      <span className={`size-1 rounded-full ${on ? 'bg-spark-cyan shadow-[0_0_6px_var(--color-spark-cyan)]' : 'bg-slate-600'}`} />
      {name}
    </span>
  )
}

function EyeIcon({className}: {className?: string}) {
  return (
    <svg viewBox="0 0 20 20" className={className} aria-hidden fill="none" stroke="currentColor" strokeWidth="1.5">
      <path d="M1.8 10S5 4.2 10 4.2 18.2 10 18.2 10 15 15.8 10 15.8 1.8 10 1.8 10Z" strokeLinejoin="round" />
      <circle cx="10" cy="10" r="2.6" />
    </svg>
  )
}

function Thumbnail({capture}: {capture: Capture}) {
  return (
    <figure className="relative w-fit overflow-hidden rounded-lg border border-spark-cyan/25 bg-black/40">
      <img src={capture.dataUrl} alt="Capture d'écran" className="block max-h-28 max-w-full" draggable={false} />
      <figcaption className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-2 pt-4 pb-1 font-mono text-[8.5px] tracking-[0.15em] text-spark-cyan/90">
        {capture.width}×{capture.height} · {Math.round(capture.bytes / 1024)} KO · {capture.millis} MS
      </figcaption>
    </figure>
  )
}

// Stays mounted while closed so the conversation and scroll survive;
// `hidden` removes it from the layout, hence from the interactive regions.
export function ChatPanel({
  open,
  speaking,
  waiting,
  aiReady,
  seeScreen,
  listening,
  partial,
  messages,
  onSend,
  onToggleScreen,
  onOpenSettings,
  onStop,
  onClose,
  toggleLabel,
}: Props) {
  const input = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLOListElement>(null)
  const [draft, setDraft] = useState('')

  useEffect(() => {
    if (open) input.current?.focus()
  }, [open])

  // Follow the stream: scroll on new messages and as the last one grows.
  const last = messages[messages.length - 1]
  useEffect(() => {
    list.current?.scrollTo({top: list.current.scrollHeight})
  }, [messages.length, last?.text, waiting, open, partial, listening])

  function submit(e: FormEvent) {
    e.preventDefault()
    const text = draft.trim()
    if (!text && !seeScreen) return
    setDraft('')
    onSend(text)
  }

  return (
    <section
      data-interactive
      hidden={!open}
      onKeyDown={(e) => e.key === 'Escape' && onClose()}
      className="relative flex h-[30rem] w-[22rem] animate-panel-in flex-col rounded-[14px] border border-white/[0.07] bg-panel text-slate-100 shadow-[0_24px_64px_-12px_rgb(0_0_0/0.75),0_0_0_1px_color-mix(in_oklab,var(--color-spark-cyan)_5%,transparent)] backdrop-blur-2xl"
    >
      <HudCorners />

      <header className="flex items-center gap-3 border-b border-white/[0.06] px-4 py-3">
        <SparkLogo size={26} />
        <div className="flex-1 leading-none">
          <div className="font-mono text-[11px] font-medium tracking-[0.32em]">A.R.I.A</div>
          <div className="mt-1.5 flex items-center gap-1.5 font-mono text-[9px] tracking-[0.22em] text-spark-cyan/80">
            {speaking ? <Waveform active /> : <span className="size-1.5 rounded-full bg-spark-cyan shadow-[0_0_8px_var(--color-spark-cyan)]" />}
            {speaking ? 'PARLE' : waiting ? 'ANALYSE' : 'EN LIGNE'}
          </div>
        </div>
        {(speaking || waiting) && (
          <button
            onClick={onStop}
            title="Arrêter A.R.I.A"
            className="flex items-center gap-1.5 rounded-md border border-white/10 px-2 py-0.5 font-mono text-[9.5px] tracking-[0.15em] text-slate-300 transition-colors hover:border-rose-400/40 hover:text-rose-200"
          >
            <span className="size-1.5 rounded-[1px] bg-current" />
            STOP
          </button>
        )}
        <kbd className="rounded border border-white/10 px-1.5 py-0.5 font-mono text-[9.5px] text-slate-400">{toggleLabel}</kbd>
        <button
          onClick={onOpenSettings}
          aria-label="Paramètres"
          title="Paramètres"
          className="grid size-6 place-items-center rounded-md text-slate-400 transition-colors hover:bg-white/5 hover:text-slate-100"
        >
          <svg viewBox="0 0 20 20" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" aria-hidden>
            <circle cx="10" cy="10" r="2.5" />
            <path d="M10 2v2.2M10 15.8V18M2 10h2.2M15.8 10H18M4.3 4.3l1.6 1.6M14.1 14.1l1.6 1.6M4.3 15.7l1.6-1.6M14.1 5.9l1.6-1.6" />
          </svg>
        </button>
        <button
          onClick={onClose}
          aria-label="Fermer"
          className="grid size-6 place-items-center rounded-md text-slate-400 transition-colors hover:bg-white/5 hover:text-slate-100"
        >
          <svg viewBox="0 0 12 12" className="size-3" aria-hidden>
            <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
          </svg>
        </button>
      </header>

      <ol ref={list} className="hud-scroll flex-1 space-y-4 overflow-y-auto px-4 py-4 select-text">
        {messages.map((m) =>
          m.from === 'aria' ? (
            <li key={m.id} className="relative pl-3.5">
              <span
                className={`absolute top-1 bottom-1 left-0 w-px bg-gradient-to-b ${
                  m.error ? 'from-rose-400 to-rose-400/20' : 'from-spark-cyan to-spark-violet/30'
                }`}
              />
              <div className={`mb-1 font-mono text-[9px] tracking-[0.22em] ${m.error ? 'text-rose-300/80' : 'text-spark-cyan/70'}`}>
                ARIA · {time(m.at)}
              </div>
              <p className={`text-[13.5px] leading-relaxed ${m.error ? 'text-rose-200/90' : 'text-slate-200'}`}>
                {m.text}
                {m.streaming && (
                  <span className="ml-0.5 inline-block h-[1em] w-[2px] translate-y-[2px] animate-blink bg-spark-cyan" />
                )}
              </p>
            </li>
          ) : (
            <li key={m.id} className="flex flex-col items-end gap-1.5">
              {m.image && <Thumbnail capture={m.image} />}
              {m.text && (
                <p className="max-w-[85%] rounded-xl rounded-br-sm border border-white/[0.07] bg-gradient-to-br from-spark-cyan/[0.14] to-spark-violet/[0.14] px-3 py-2 text-[13.5px] leading-relaxed">
                  {m.voice && (
                    <svg viewBox="0 0 20 20" className="mr-1.5 inline size-3 -translate-y-px text-spark-cyan" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-label="Message vocal">
                      <rect x="7.5" y="2.5" width="5" height="9" rx="2.5" />
                      <path d="M4.5 9.5a5.5 5.5 0 0011 0M10 15v2.5" />
                    </svg>
                  )}
                  {m.text}
                </p>
              )}
              <span className="font-mono text-[9px] tracking-[0.18em] text-slate-500">{time(m.at)}</span>
            </li>
          ),
        )}
        {listening !== 'idle' && (
          <li className="flex flex-col items-end gap-1">
            <p className="max-w-[85%] rounded-xl rounded-br-sm border border-dashed border-spark-cyan/40 px-3 py-2 text-[13.5px] leading-relaxed text-slate-300">
              {partial || <span className="text-slate-500">J’écoute…</span>}
            </p>
            <span className="flex items-center gap-1.5 font-mono text-[9px] tracking-[0.2em] text-spark-cyan/80">
              <Waveform active />
              {listening === 'listening' ? 'ÉCOUTE' : 'TRANSCRIPTION'}
            </span>
          </li>
        )}
        {waiting && (
          <li className="relative flex items-center gap-2 pl-3.5 font-mono text-[9.5px] tracking-[0.25em] text-spark-cyan/80">
            <span className="absolute top-0 bottom-0 left-0 w-px bg-gradient-to-b from-spark-cyan to-spark-violet/30" />
            <Waveform active />
            {seeScreen ? 'ANALYSE DE L’ÉCRAN' : 'RÉFLEXION'}
          </li>
        )}
      </ol>

      <form onSubmit={submit} className="px-3 pb-2">
        <div className="flex items-center gap-2 rounded-lg border border-white/[0.08] bg-black/30 pr-3 pl-1.5 transition-[border-color,box-shadow] focus-within:border-spark-cyan/50 focus-within:shadow-[0_0_0_3px_color-mix(in_oklab,var(--color-spark-cyan)_8%,transparent)]">
          <button
            type="button"
            onClick={onToggleScreen}
            aria-pressed={seeScreen}
            aria-label={seeScreen ? 'A.R.I.A voit ton écran' : 'Vision coupée'}
            title={seeScreen ? 'Vision active : chaque question part avec une capture' : 'Vision coupée'}
            className={`relative grid size-7 place-items-center rounded-md transition-colors hover:bg-white/5 ${
              seeScreen ? 'text-spark-cyan' : 'text-slate-500'
            }`}
          >
            <EyeIcon className="size-4" />
            {!seeScreen && <span className="absolute h-px w-4 rotate-45 bg-current" />}
          </button>
          <input
            ref={input}
            autoComplete="off"
            autoCorrect="off"
            autoCapitalize="off"
            spellCheck={false}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={seeScreen ? 'Que dois-je faire ici ?' : 'Demande à A.R.I.A…'}
            className="h-10 flex-1 bg-transparent text-[13.5px] outline-none placeholder:text-slate-500"
          />
          <kbd className="font-mono text-[10px] text-slate-500">↵</kbd>
        </div>
      </form>

      <footer className="flex items-center gap-4 px-4 pb-3 font-mono text-[8.5px] tracking-[0.22em]">
        <Module name="VISION" on={seeScreen} />
        <Module name="IA" on={aiReady} />
        <Module name="VOIX" on />
        <span className="ml-auto text-slate-600">ÉCHAP</span>
      </footer>
    </section>
  )
}
