import {useId} from 'react'

// Same geometry as build/brand/logo.svg (64×64 grid).
const STAR = 'M32 8.5Q33.9 30.1 52.5 32Q33.9 33.9 32 55.5Q30.1 33.9 11.5 32Q30.1 30.1 32 8.5Z'

// Follows the accent color (theme tokens), defaulting to the brand colors.
function Gradient({id}: {id: string}) {
  return (
    <linearGradient id={id} x1="12" y1="10" x2="52" y2="54" gradientUnits="userSpaceOnUse">
      <stop offset="0" style={{stopColor: 'var(--color-spark-teal, #5eead4)'}} />
      <stop offset=".5" style={{stopColor: 'var(--color-spark-cyan, #22d3ee)'}} />
      <stop offset="1" style={{stopColor: 'var(--color-spark-violet, #8b5cf6)'}} />
    </linearGradient>
  )
}

type Props = {size?: number; className?: string; style?: React.CSSProperties}

/** The three HUD arcs (one per letter of A.R.I.A), rotatable on its own. */
export function LogoRing({size = 64, className}: Props) {
  const id = useId()
  return (
    <svg viewBox="0 0 64 64" width={size} height={size} className={className} aria-hidden>
      <defs>
        <Gradient id={id} />
      </defs>
      <circle
        cx="32" cy="32" r="28" fill="none" stroke={`url(#${id})`} strokeWidth="2.2"
        strokeLinecap="round" strokeDasharray="44.6 14" transform="rotate(-62 32 32)"
      />
      <circle cx="32" cy="32" r="22.5" fill="none" stroke="var(--color-spark-cyan, #22d3ee)" strokeOpacity=".25" strokeWidth=".8" />
    </svg>
  )
}

/** The spark itself. */
export function LogoStar({size = 64, className, style}: Props) {
  const id = useId()
  return (
    <svg viewBox="0 0 64 64" width={size} height={size} className={className} style={style} aria-hidden>
      <defs>
        <Gradient id={id} />
      </defs>
      <path d={STAR} fill={`url(#${id})`} />
      <circle cx="32" cy="32" r="4" fill="var(--color-spark-cyan, #22d3ee)" fillOpacity=".35" />
      <circle cx="32" cy="32" r="1.9" fill="#ecfeff" />
    </svg>
  )
}

export function SparkLogo({size = 64, className = ''}: Props) {
  return (
    <span className={`relative inline-block shrink-0 ${className}`} style={{width: size, height: size}}>
      <LogoRing size={size} className="absolute inset-0" />
      <LogoStar size={size} className="absolute inset-0" />
    </span>
  )
}
