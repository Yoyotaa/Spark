import {useEffect, useState, type ReactNode} from 'react'
import * as bridge from '../../lib/bridge'
import type {AIStatus} from '../../lib/bridge'
import type {Settings} from '../../lib/settings'
import {HudCorners} from '../HudCorners'
import {SparkLogo} from '../SparkLogo'
import {
  AboutSection,
  AdvancedSection,
  AISection,
  AppearanceSection,
  HotkeysSection,
  MicSection,
  PrivacySection,
  VisionSection,
  VoiceSection,
} from './sections'

export type SaveState = 'idle' | 'saving' | 'saved' | 'error'

type Props = {
  settings: Settings
  update: (fn: (draft: Settings) => void) => void
  saveState: SaveState
  saveError: string | null
  hotkeyError?: string
  onReset: () => void
  onResetOrb: () => void
  onCleared: () => void
  onClose: () => void
}

const icon = (d: ReactNode) => (
  <svg viewBox="0 0 20 20" className="size-4" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
    {d}
  </svg>
)

const NAV = [
  {id: 'ai', label: 'Intelligence', desc: 'Fournisseur, modèle, clé API et personnalité d’A.R.I.A.', icon: icon(<><path d="M10 2.5l1.6 4.4 4.4 1.6-4.4 1.6L10 14.5l-1.6-4.4L4 8.5l4.4-1.6z" /><path d="M15.5 13l.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z" /></>)},
  {id: 'vision', label: 'Vision', desc: 'Ce que voit A.R.I.A et avec quelle précision.', icon: icon(<><path d="M1.8 10S5 4.2 10 4.2 18.2 10 18.2 10 15 15.8 10 15.8 1.8 10 1.8 10Z" /><circle cx="10" cy="10" r="2.6" /></>)},
  {id: 'voice', label: 'Voix', desc: 'Comment A.R.I.A te parle.', icon: icon(<><path d="M3 8v4h3l4 3V5L6 8z" /><path d="M13.5 7.5a3.5 3.5 0 010 5M15.5 5.5a6.4 6.4 0 010 9" /></>)},
  {id: 'mic', label: 'Micro', desc: 'Parler à A.R.I.A : push-to-talk ou écoute continue.', icon: icon(<><rect x="7.5" y="2.5" width="5" height="9" rx="2.5" /><path d="M4.5 9.5a5.5 5.5 0 0011 0M10 15v2.5" /></>)},
  {id: 'hotkeys', label: 'Raccourcis', desc: 'Les combinaisons qui marchent même en jeu.', icon: icon(<><rect x="2" y="5" width="16" height="10" rx="2" /><path d="M5 8.5h1M8 8.5h1M11 8.5h1M14 8.5h1M6 11.5h8" /></>)},
  {id: 'appearance', label: 'Apparence', desc: 'Couleurs, transparence et bouton.', icon: icon(<><circle cx="10" cy="10" r="7.5" /><path d="M10 2.5a7.5 7.5 0 000 15z" fill="currentColor" fillOpacity=".25" /></>)},
  {id: 'privacy', label: 'Confidentialité', desc: 'Ce qui est envoyé, ce qui reste ici.', icon: icon(<><path d="M10 2.5l6 2.5v4.5c0 4-2.6 6.6-6 8-3.4-1.4-6-4-6-8V5z" /><path d="M7.5 10l1.8 1.8 3.2-3.6" /></>)},
  {id: 'advanced', label: 'Avancé', desc: 'Performance, debug et réinitialisation.', icon: icon(<><circle cx="10" cy="10" r="2.5" /><path d="M10 2v2.2M10 15.8V18M2 10h2.2M15.8 10H18M4.3 4.3l1.6 1.6M14.1 14.1l1.6 1.6M4.3 15.7l1.6-1.6M14.1 5.9l1.6-1.6" /></>)},
  {id: 'about', label: 'À propos', desc: 'Version et emplacements.', icon: icon(<><circle cx="10" cy="10" r="7.5" /><path d="M10 9v5M10 6.5v.01" /></>)},
] as const

type SectionId = (typeof NAV)[number]['id']

export function SettingsPanel(props: Props) {
  const {settings: s, update, saveState, saveError, onClose} = props
  const [section, setSection] = useState<SectionId>('ai')
  const [status, setStatus] = useState<AIStatus | null>(null)

  // Refresh the connection state whenever AI settings may have changed it.
  const aiKey = JSON.stringify(s.ai)
  useEffect(() => {
    if (saveState === 'saving') return
    void bridge.getAIStatus().then(setStatus)
  }, [aiKey, saveState])

  const current = NAV.find((n) => n.id === section)!

  return (
    <div
      data-interactive
      className="pointer-events-auto fixed inset-0 grid animate-[panel-in_200ms_ease-out_both] place-items-center bg-black/45 backdrop-blur-[3px]"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
      onKeyDown={(e) => e.key === 'Escape' && onClose()}
    >
      <div
        role="dialog"
        aria-label="Paramètres de Spark"
        className="relative grid h-[min(640px,88vh)] w-[min(940px,94vw)] grid-cols-[230px_1fr] grid-rows-[minmax(0,1fr)] overflow-hidden rounded-[18px] border border-white/[0.08] bg-panel text-slate-100 shadow-[0_40px_120px_-20px_rgb(0_0_0/0.85)] backdrop-blur-2xl"
      >
        <HudCorners />

        <nav className="flex flex-col border-r border-white/[0.06] bg-black/20 p-3">
          <div className="flex items-center gap-2.5 px-2 pt-1 pb-5">
            <SparkLogo size={28} />
            <div className="leading-none">
              <div className="text-[14px] font-semibold tracking-wide">Spark</div>
              <div className="mt-1 font-mono text-[9px] tracking-[0.28em] text-spark-cyan/80">PARAMÈTRES</div>
            </div>
          </div>
          <ul className="space-y-0.5">
            {NAV.map((n) => (
              <li key={n.id}>
                <button
                  type="button"
                  onClick={() => setSection(n.id)}
                  className={`relative flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-[12.5px] transition-colors ${
                    section === n.id ? 'bg-white/[0.06] text-slate-50' : 'text-slate-400 hover:bg-white/[0.03] hover:text-slate-200'
                  }`}
                >
                  {section === n.id && <span className="absolute top-2 bottom-2 left-0 w-[2px] rounded-full bg-spark-cyan shadow-[0_0_8px_var(--color-spark-cyan)]" />}
                  <span className={section === n.id ? 'text-spark-cyan' : ''}>{n.icon}</span>
                  {n.label}
                </button>
              </li>
            ))}
          </ul>
          <div className="mt-auto px-2 pt-4 font-mono text-[9px] tracking-[0.2em] text-slate-600">ÉCHAP POUR FERMER</div>
        </nav>

        <div className="flex min-h-0 flex-col">
          <header className="flex items-start justify-between gap-4 border-b border-white/[0.06] px-7 pt-6 pb-4">
            <div>
              <h2 className="text-[19px] font-semibold tracking-tight">{current.label}</h2>
              <p className="mt-1 text-[12px] text-slate-400">{current.desc}</p>
            </div>
            <div className="flex items-center gap-3">
              <span
                className={`font-mono text-[10px] tracking-[0.18em] transition-opacity ${
                  saveState === 'error' ? 'text-rose-300' : saveState === 'idle' ? 'opacity-0' : 'text-spark-cyan/80'
                }`}
                title={saveError ?? undefined}
              >
                {saveState === 'saving' ? 'ENREGISTREMENT…' : saveState === 'error' ? 'ÉCHEC' : 'ENREGISTRÉ ✓'}
              </span>
              <button
                type="button"
                onClick={onClose}
                aria-label="Fermer les paramètres"
                className="grid size-7 place-items-center rounded-lg text-slate-400 transition-colors hover:bg-white/5 hover:text-slate-100"
              >
                <svg viewBox="0 0 12 12" className="size-3" aria-hidden>
                  <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
                </svg>
              </button>
            </div>
          </header>

          <div className="hud-scroll min-h-0 flex-1 overflow-y-auto px-7 py-6 select-text">
            {saveError && (
              <div className="mb-4 rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-[12px] text-rose-200">{saveError}</div>
            )}
            {section === 'ai' && <AISection s={s} update={update} status={status} onStatus={setStatus} />}
            {section === 'vision' && <VisionSection s={s} update={update} />}
            {section === 'voice' && <VoiceSection s={s} update={update} />}
            {section === 'mic' && <MicSection s={s} update={update} />}
            {section === 'hotkeys' && <HotkeysSection s={s} update={update} hotkeyError={props.hotkeyError} />}
            {section === 'appearance' && <AppearanceSection s={s} update={update} onResetOrb={props.onResetOrb} />}
            {section === 'privacy' && <PrivacySection s={s} update={update} onCleared={props.onCleared} />}
            {section === 'advanced' && <AdvancedSection s={s} update={update} onReset={props.onReset} />}
            {section === 'about' && <AboutSection />}
          </div>
        </div>
      </div>
    </div>
  )
}
