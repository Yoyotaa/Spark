import {useEffect, useRef, useState, type FormEvent} from 'react'
import {HudCorners} from './HudCorners'
import {SparkLogo} from './SparkLogo'
import {Waveform} from './Waveform'

export type Message = {id: number; from: 'user' | 'aria'; text: string; at: Date}

type Props = {
  open: boolean
  speaking: boolean
  messages: Message[]
  onSend: (text: string) => void
  onClose: () => void
}

const time = (d: Date) => d.toLocaleTimeString('fr-FR', {hour: '2-digit', minute: '2-digit'})

function Module({name, on}: {name: string; on: boolean}) {
  return (
    <span className={`flex items-center gap-1.5 ${on ? 'text-spark-cyan/80' : 'text-slate-600'}`}>
      <span className={`size-1 rounded-full ${on ? 'bg-spark-cyan shadow-[0_0_6px_#22d3ee]' : 'bg-slate-600'}`} />
      {name}
    </span>
  )
}

// Stays mounted while closed so the conversation and scroll survive;
// `hidden` removes it from the layout, hence from the interactive regions.
export function ChatPanel({open, speaking, messages, onSend, onClose}: Props) {
  const input = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLOListElement>(null)
  const [draft, setDraft] = useState('')

  useEffect(() => {
    if (open) input.current?.focus()
  }, [open])

  useEffect(() => {
    list.current?.scrollTo({top: list.current.scrollHeight, behavior: 'smooth'})
  }, [messages.length, open])

  function submit(e: FormEvent) {
    e.preventDefault()
    const text = draft.trim()
    if (!text) return
    setDraft('')
    onSend(text)
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
            <li key={m.id} className="flex flex-col items-end">
              <p className="max-w-[85%] rounded-xl rounded-br-sm border border-white/[0.07] bg-gradient-to-br from-spark-cyan/[0.14] to-spark-violet/[0.14] px-3 py-2 text-[13.5px] leading-relaxed">
                {m.text}
              </p>
              <span className="mt-1 font-mono text-[9px] tracking-[0.18em] text-slate-500">{time(m.at)}</span>
            </li>
          ),
        )}
      </ol>

      <form onSubmit={submit} className="px-3 pb-2">
        <div className="flex items-center gap-2 rounded-lg border border-white/[0.08] bg-black/30 px-3 transition-[border-color,box-shadow] focus-within:border-spark-cyan/50 focus-within:shadow-[0_0_0_3px_rgb(34_211_238/0.08)]">
          <span className="font-mono text-sm text-spark-cyan">›</span>
          <input
            ref={input}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Demande à A.R.I.A…"
            className="h-10 flex-1 bg-transparent text-[13.5px] outline-none placeholder:text-slate-500"
          />
          <kbd className="font-mono text-[10px] text-slate-500">↵</kbd>
        </div>
      </form>

      <footer className="flex items-center gap-4 px-4 pb-3 font-mono text-[8.5px] tracking-[0.22em]">
        <Module name="VISION" on={false} />
        <Module name="IA" on={false} />
        <Module name="VOIX" on />
        <span className="ml-auto text-slate-600">ÉCHAP</span>
      </footer>
    </section>
  )
}
