type Props = {
  active: boolean
  onToggle: () => void
}

export function AriaOrb({active, onToggle}: Props) {
  return (
    <button
      data-interactive
      onClick={onToggle}
      aria-label={active ? 'Fermer A.R.I.A' : 'Ouvrir A.R.I.A'}
      className="group relative grid size-14 cursor-pointer place-items-center rounded-full border border-aria/40 bg-panel shadow-[0_0_24px_rgb(56_189_248/0.35)] backdrop-blur-md transition-transform duration-200 hover:scale-105 active:scale-95"
    >
      <span className="absolute inset-2 animate-ping rounded-full bg-aria/20 [animation-duration:2.5s]" />
      <span
        className={`size-5 rounded-full bg-gradient-to-br from-aria-glow to-aria transition-all duration-300 ${
          active ? 'scale-75 opacity-70' : 'scale-100'
        }`}
      />
    </button>
  )
}
