import {useEffect, useRef, useState, type FormEvent} from 'react'
import {captureScreen, type Capture} from '../lib/bridge'
import {HudCorners} from './HudCorners'
import {SparkLogo} from './SparkLogo'
import {Waveform} from './Waveform'

export type Message = {
  id: number
  from: 'user' | 'aria'
  text: string
  at: Date
  image?: Capture
}

type Props = {
  open: boolean
  speaking: boolean
  messages: Message[]
  onSend: (text: string, image?: Capture) => void
  onClose: () => void
}

const time = (d: Date) => d.toLocaleTimeString('fr-FR', {hour: '2-digit', minute: '2-digit'})

const errorText = (e: unknown) => (e instanceof Error ? e.message : String(e))

function Module({name, on}: {name: string; on: boolean}) {
  return (
    <span className={`flex items-center gap-1.5 ${on ? 'text-spark-cyan/80' : 'text-slate-600'}`}>
      <span className={`size-1 rounded-full ${on ? 'bg-spark-cyan shadow-[0_0_6px_#22d3ee]' : 'bg-slate-600'}`} />
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

function Thumbnail({capture, onRemove}: {capture: Capture; onRemove?: () => void}) {
  return (
    <figure className="relative w-fit overflow-hidden rounded-lg border border-spark-cyan/25 bg-black/40">
      <img src={capture.dataUrl} alt="Capture d'écran" className="block max-h-28 max-w-full" draggable={false} />
      <figcaption className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-2 pt-4 pb-1 font-mono text-[8.5px] tracking-[0.15em] text-spark-cyan/90">
        {capture.width}×{capture.height} · {Math.round(capture.bytes / 1024)} KO · {capture.millis} MS
      </figcaption>
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label="Retirer la capture"
          className="absolute top-1 right-1 grid size-5 place-items-center rounded bg-black/60 text-slate-300 hover:text-white"
        >
          <svg viewBox="0 0 12 12" className="size-2.5" aria-hidden>
            <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          </svg>
        </button>
      )}
    </figure>
  )
}

// Stays mounted while closed so the conversation and scroll survive;
// `hidden` removes it from the layout, hence from the interactive regions.
export function ChatPanel({open, speaking, messages, onSend, onClose}: Props) {
  const input = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLOListElement>(null)
  const [draft, setDraft] = useState('')
  const [attachment, setAttachment] = useState<Capture | null>(null)
  const [capturing, setCapturing] = useState(false)
  const [visionOk, setVisionOk] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (open) input.current?.focus()
  }, [open])

  useEffect(() => {
    list.current?.scrollTo({top: list.current.scrollHeight, behavior: 'smooth'})
  }, [messages.length, open])

  async function capture() {
    if (capturing) return
    setCapturing(true)
    setError(null)
    try {
      setAttachment(await captureScreen())
      setVisionOk(true)
    } catch (e) {
      setError(errorText(e))
      setVisionOk(false)
    } finally {
      setCapturing(false)
      input.current?.focus()
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    const text = draft.trim()
    if (!text && !attachment) return
    setDraft('')
    setAttachment(null)
    onSend(text, attachment ?? undefined)
  }

  return (
    <section
      data-interactive
      hidden={!open}
      onKeyDown={(e) => e.key === 'Escape' && onClose()}
      className="relative flex h-[30rem] w-[22rem] animate-panel-in flex-col rounded-[14px] border border-white/[0.07] bg-panel text-slate-100 shadow-[0_24px_64px_-12px_rgb(0_0_0/0.75),0_0_0_1px_rgb(34_211_238/0.05)] backdrop-blur-2xl"
    >
      <HudCorners />

      <header className="flex items-center gap-3 border-b border-white/[0.06] px-4 py-3">
        <SparkLogo size={26} />
        <div className="flex-1 leading-none">
          <div className="font-mono text-[11px] font-medium tracking-[0.32em]">A.R.I.A</div>
          <div className="mt-1.5 flex items-center gap-1.5 font-mono text-[9px] tracking-[0.22em] text-spark-cyan/80">
            {speaking ? <Waveform active /> : <span className="size-1.5 rounded-full bg-spark-cyan shadow-[0_0_8px_#22d3ee]" />}
            {speaking ? 'PARLE' : 'EN LIGNE'}
          </div>
        </div>
        <kbd className="rounded border border-white/10 px-1.5 py-0.5 font-mono text-[9.5px] text-slate-400">⌥⇧A</kbd>
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
              <span className="absolute top-1 bottom-1 left-0 w-px bg-gradient-to-b from-spark-cyan to-spark-violet/30" />
              <div className="mb-1 font-mono text-[9px] tracking-[0.22em] text-spark-cyan/70">
                ARIA · {time(m.at)}
              </div>
              <p className="text-[13.5px] leading-relaxed text-slate-200">{m.text}</p>
            </li>
          ) : (
            <li key={m.id} className="flex flex-col items-end gap-1.5">
              {m.image && <Thumbnail capture={m.image} />}
              {m.text && (
                <p className="max-w-[85%] rounded-xl rounded-br-sm border border-white/[0.07] bg-gradient-to-br from-spark-cyan/[0.14] to-spark-violet/[0.14] px-3 py-2 text-[13.5px] leading-relaxed">
                  {m.text}
                </p>
              )}
              <span className="font-mono text-[9px] tracking-[0.18em] text-slate-500">{time(m.at)}</span>
            </li>
          ),
        )}
      </ol>

      <form onSubmit={submit} className="space-y-2 px-3 pb-2">
        {attachment && <Thumbnail capture={attachment} onRemove={() => setAttachment(null)} />}
        {error && <p className="font-mono text-[10px] leading-snug text-rose-300/90">{error}</p>}
        <div className="flex items-center gap-2 rounded-lg border border-white/[0.08] bg-black/30 pr-3 pl-1.5 transition-[border-color,box-shadow] focus-within:border-spark-cyan/50 focus-within:shadow-[0_0_0_3px_rgb(34_211_238/0.08)]">
          <button
            type="button"
            onClick={capture}
            disabled={capturing}
            aria-label="Joindre une capture de l'écran"
            title="Montrer mon écran à A.R.I.A"
            className={`grid size-7 place-items-center rounded-md transition-colors hover:bg-white/5 ${
              attachment ? 'text-spark-cyan' : 'text-slate-400 hover:text-spark-cyan'
            } ${capturing ? 'animate-pulse' : ''}`}
          >
            <EyeIcon className="size-4" />
          </button>
          <input
            ref={input}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={attachment ? 'Que veux-tu savoir sur ton écran ?' : 'Demande à A.R.I.A…'}
            className="h-10 flex-1 bg-transparent text-[13.5px] outline-none placeholder:text-slate-500"
          />
          <kbd className="font-mono text-[10px] text-slate-500">↵</kbd>
        </div>
      </form>

      <footer className="flex items-center gap-4 px-4 pb-3 font-mono text-[8.5px] tracking-[0.22em]">
        <Module name="VISION" on={visionOk} />
        <Module name="IA" on={false} />
        <Module name="VOIX" on />
        <span className="ml-auto text-slate-600">ÉCHAP</span>
      </footer>
    </section>
  )
}
