// Accent brackets on the four corners of a HUD panel.
export function HudCorners() {
  const base = 'pointer-events-none absolute size-3.5 border-spark-cyan/70'
  return (
    <>
      <span className={`${base} -top-px -left-px rounded-tl-[14px] border-t border-l`} />
      <span className={`${base} -top-px -right-px rounded-tr-[14px] border-t border-r`} />
      <span className={`${base} -bottom-px -left-px rounded-bl-[14px] border-b border-l`} />
      <span className={`${base} -right-px -bottom-px rounded-br-[14px] border-r border-b`} />
      <span className="pointer-events-none absolute inset-x-8 -top-px h-px bg-gradient-to-r from-transparent via-spark-cyan/70 to-transparent" />
    </>
  )
}
