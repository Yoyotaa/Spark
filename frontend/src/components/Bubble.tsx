import {useEffect, useState} from 'react'
import {Waveform} from './Waveform'

type Props = {
  text: string
  speaking: boolean
  side: 'left' | 'right'
}

const CHAR_MS = 28

/** Transient message next to the orb (greeting, proactive remarks). */
export function Bubble({text, speaking, side}: Props) {
  const [shown, setShown] = useState(0)

  useEffect(() => {
    setShown(0)
    const id = setInterval(() => {
      setShown((n) => {
        if (n >= text.length) clearInterval(id)
        return Math.min(n + 1, text.length)
      })
    }, CHAR_MS)
    return () => clearInterval(id)
  }, [text])

  const typing = shown < text.length
  return (
    <div
      className="pointer-events-none w-72 animate-bubble-in rounded-[14px] border border-white/10 bg-panel px-4 py-3 shadow-[0_16px_48px_-12px_rgb(0_0_0/0.7)] backdrop-blur-xl"
      style={{'--bubble-from': side === 'left' ? '8px' : '-8px'} as React.CSSProperties}
    >
      <div className="mb-1.5 flex items-center justify-between">
        <span className="font-mono text-[10px] tracking-[0.3em] text-spark-cyan">A.R.I.A</span>
        <Waveform active={speaking} />
      </div>
      <p className="text-[13.5px] leading-relaxed text-slate-100">
        {text.slice(0, shown)}
        {typing && <span className="ml-px inline-block h-[1em] w-[2px] translate-y-[2px] animate-blink bg-spark-cyan" />}
      </p>
    </div>
  )
}
