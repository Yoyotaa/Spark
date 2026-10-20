import type {PointerEvent} from 'react'
import {LogoRing, LogoStar} from './SparkLogo'

export const ORB_SIZE = 60

type Props = {
  open: boolean
  speaking: boolean
  handlers: {
    onPointerDown: (e: PointerEvent<HTMLElement>) => void
    onPointerMove: (e: PointerEvent<HTMLElement>) => void
    onPointerUp: (e: PointerEvent<HTMLElement>) => void
    onPointerCancel: (e: PointerEvent<HTMLElement>) => void
  }
}

/**
 * A.R.I.A's button. Idle, nothing animates (zero GPU work while you play);
 * the ring only spins while she speaks or the chat is open.
 */
export function Orb({open, speaking, handlers}: Props) {
  const active = open || speaking
  return (
    <button
      data-interactive
      {...handlers}
      aria-label={open ? 'Fermer A.R.I.A' : 'Ouvrir A.R.I.A'}
      title="A.R.I.A · ⌥⇧A  —  glisser pour déplacer  —  masquer : ⌥⇧H"
      className="group relative grid cursor-grab touch-none place-items-center rounded-full outline-none data-dragging:cursor-grabbing"
      style={{width: ORB_SIZE, height: ORB_SIZE}}
    >
      {/* one-shot boot shockwave */}
      <span className="pointer-events-none absolute inset-0 animate-shockwave rounded-full border border-spark-cyan/60" />

      <span className="absolute inset-0 animate-orb-boot">
        {/* speaking halo */}
        {speaking && (
          <>
            <span className="absolute inset-0 animate-halo rounded-full border border-spark-cyan/70" />
            <span className="absolute inset-0 animate-halo rounded-full border border-spark-violet/60 [animation-delay:0.9s]" />
          </>
        )}

        {/* glass core */}
        <span
          className={`absolute inset-[5px] rounded-full border bg-[radial-gradient(circle_at_35%_30%,rgb(34_211_238/0.22),rgb(8_11_20/0.92)_62%)] backdrop-blur-md transition-all duration-300 group-hover:shadow-[0_0_28px_rgb(34_211_238/0.45)] group-data-dragging:scale-110 ${
            active
              ? 'border-spark-cyan/50 shadow-[0_0_32px_rgb(34_211_238/0.5)]'
              : 'border-white/10 shadow-[0_0_18px_rgb(34_211_238/0.22),0_8px_24px_rgb(0_0_0/0.5)]'
          }`}
        />

        <LogoRing
          size={ORB_SIZE}
          className={`absolute inset-0 transition-opacity duration-300 ${
            active ? 'animate-ring-spin opacity-100' : 'opacity-70 group-hover:opacity-100'
          } ${speaking ? '[animation-duration:1.4s]' : ''}`}
        />
        <LogoStar
          size={30}
          className={`absolute inset-[15px] transition-transform duration-300 group-hover:scale-110 ${open ? 'rotate-45' : ''}`}
        />
      </span>
    </button>
  )
}
