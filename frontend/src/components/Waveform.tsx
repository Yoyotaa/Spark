// Five bars bouncing while A.R.I.A speaks (transform-only animation).
export function Waveform({active}: {active: boolean}) {
  return (
    <span className="flex h-3 items-center gap-[2px]" aria-hidden>
      {[0, 0.15, 0.3, 0.1, 0.25].map((delay, i) => (
        <span
          key={i}
          className={`h-full w-[2px] origin-center rounded-full bg-spark-cyan ${
            active ? 'animate-wave' : 'scale-y-25 opacity-50'
          }`}
          style={{animationDelay: `${delay}s`}}
        />
      ))}
    </span>
  )
}
