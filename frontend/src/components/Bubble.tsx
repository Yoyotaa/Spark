import {useEffect, useRef, useState} from 'react'
import {Waveform} from './Waveform'

export type BubbleContent =
  | {kind: 'say'; text: string}
  | {kind: 'listen'; text: string; transcribing: boolean}
  | {kind: 'answer'; turnId: number; question: string; text: string; done: boolean}
  | {kind: 'error'; text: string}

type Props = {
  content: BubbleContent
  speaking: boolean
  side: 'left' | 'right'
}

const CHAR_MS = 22

/**
 * Typewriter that keeps going when the text grows (streamed answers) and
 * only restarts when it is replaced.
 */
function useTypewriter(text: string, instant: boolean) {
  const [shown, setShown] = useState(instant ? text.length : 0)
  const prev = useRef(text)

  useEffect(() => {
    if (!text.startsWith(prev.current)) setShown(instant ? text.length : 0)
    prev.current = text
  }, [text, instant])

  useEffect(() => {
    if (instant) {
      setShown(text.length)
      return
    }
    if (shown >= text.length) return
    const id = setTimeout(() => setShown((n) => Math.min(n + 1, text.length)), CHAR_MS)
    return () => clearTimeout(id)
  }, [shown, text, instant])

  return text.slice(0, shown)
}

function MicIcon() {
  return (
    <svg viewBox="0 0 20 20" className="size-3" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" aria-hidden>
      <rect x="7.5" y="2.5" width="5" height="9" rx="2.5" />
      <path d="M4.5 9.5a5.5 5.5 0 0011 0M10 15v2.5" />
    </svg>
  )
}

/** Transient card next to the orb: greeting, live transcription, answers. */
export function Bubble({content, speaking, side}: Props) {
  // Live transcription is shown as is: it is already "typing".
  const typed = useTypewriter(content.text, content.kind === 'listen')
  const typing = typed.length < content.text.length || (content.kind === 'answer' && !content.done)

  const title = content.kind === 'listen' ? 'TOI' : 'A.R.I.A'
  const tone =
    content.kind === 'error'
      ? 'text-rose-300'
      : content.kind === 'listen'
        ? 'text-slate-300'
        : 'text-spark-cyan'

  return (
    <div
      className="pointer-events-none w-80 animate-bubble-in rounded-[14px] border border-white/10 bg-panel px-4 py-3 shadow-[0_16px_48px_-12px_rgb(0_0_0/0.7)] backdrop-blur-xl"
      style={{'--bubble-from': side === 'left' ? '8px' : '-8px'} as React.CSSProperties}
    >
      <div className="mb-1.5 flex items-center justify-between">
        <span className={`flex items-center gap-1.5 font-mono text-[10px] tracking-[0.3em] ${tone}`}>
          {content.kind === 'listen' && <MicIcon />}
          {title}
        </span>
        {content.kind === 'listen' ? (
          <span className="font-mono text-[9px] tracking-[0.2em] text-spark-cyan/80">
            {content.transcribing ? 'TRANSCRIPTION' : 'ÉCOUTE'}
          </span>
        ) : (
          <Waveform active={speaking} />
        )}
      </div>

      {content.kind === 'answer' && content.question && (
        <p className="mb-2 flex items-start gap-1.5 border-b border-white/[0.06] pb-2 text-[12px] leading-snug text-slate-400">
          <span className="mt-0.5 shrink-0">
            <MicIcon />
          </span>
          {content.question}
        </p>
      )}

      <p className={`text-[13.5px] leading-relaxed ${content.kind === 'error' ? 'text-rose-200' : 'text-slate-100'}`}>
        {content.kind === 'listen' && !content.text ? (
          <span className="text-slate-500">J’écoute…</span>
        ) : content.kind === 'answer' && !content.text ? (
          <span className="flex items-center gap-2 font-mono text-[10px] tracking-[0.22em] text-spark-cyan/80">
            <Waveform active /> ANALYSE
          </span>
        ) : (
          typed
        )}
        {typing && content.text && (
          <span className="ml-px inline-block h-[1em] w-[2px] translate-y-[2px] animate-blink bg-spark-cyan" />
        )}
      </p>
    </div>
  )
}
