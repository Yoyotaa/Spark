import {useEffect, useId, useRef, useState} from 'react'
import * as bridge from '../../lib/bridge'
import type {AIStatus, AppInfo, MicStatus, TestResult, Voice} from '../../lib/bridge'
import {ACCENTS, DEFAULT_MODELS, MODEL_HINTS, type Accent, type Provider, type Settings} from '../../lib/settings'
import {Badge, Button, Group, inputClass, Row, Segmented, Select, SecretField, ShortcutRecorder, Slider, Toggle} from './controls'

export type SectionProps = {
  s: Settings
  update: (fn: (draft: Settings) => void) => void
}

const errorText = (e: unknown) => (e instanceof Error ? e.message : String(e))

// ---- Intelligence ------------------------------------------------------------

const PROVIDERS: {id: Provider; name: string; tagline: string; keyUrl: string}[] = [
  {id: 'gemini', name: 'Gemini', tagline: 'Rapide, niveau gratuit', keyUrl: 'aistudio.google.com/apikey'},
  {id: 'claude', name: 'Claude', tagline: 'Analyse la plus fine', keyUrl: 'console.anthropic.com'},
  {id: 'openai', name: 'OpenAI & compatibles', tagline: 'OpenRouter, Ollama, LM Studio…', keyUrl: 'platform.openai.com/api-keys'},
]

export function AISection({s, update, status, onStatus}: SectionProps & {status: AIStatus | null; onStatus: (st: AIStatus) => void}) {
  const p = s.ai.provider
  const meta = PROVIDERS.find((x) => x.id === p)!
  const key = status?.keys[p]
  const [models, setModels] = useState<Record<string, string[]>>({})
  const [loadingModels, setLoadingModels] = useState(false)
  const [test, setTest] = useState<{busy: boolean; result?: TestResult; error?: string}>({busy: false})
  const [error, setError] = useState<string | null>(null)
  const listId = useId()

  const model = s.ai.models[p] || DEFAULT_MODELS[p]
  const suggestions = [...new Set([...(models[p] ?? []), ...MODEL_HINTS[p]])]

  async function refreshModels() {
    setLoadingModels(true)
    setError(null)
    try {
      const ids = await bridge.listModels(p)
      setModels((m) => ({...m, [p]: ids}))
    } catch (e) {
      setError(errorText(e))
    } finally {
      setLoadingModels(false)
    }
  }

  async function runTest() {
    setTest({busy: true})
    try {
      setTest({busy: false, result: await bridge.testAI(p)})
    } catch (e) {
      setTest({busy: false, error: errorText(e)})
    }
  }

  useEffect(() => setTest({busy: false}), [p])

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-3 gap-2.5">
        {PROVIDERS.map((x) => {
          const active = x.id === p
          const k = status?.keys[x.id]
          return (
            <button
              key={x.id}
              type="button"
              onClick={() => update((d) => void (d.ai.provider = x.id))}
              className={`relative rounded-xl border p-3.5 text-left transition-all ${
                active
                  ? 'border-spark-cyan/50 bg-spark-cyan/[0.08] shadow-[0_0_0_1px_color-mix(in_oklab,var(--color-spark-cyan)_25%,transparent)]'
                  : 'border-white/[0.07] bg-white/[0.02] hover:border-white/15'
              }`}
            >
              <div className="flex items-center justify-between">
                <span className={`text-[13.5px] font-semibold ${active ? 'text-spark-cyan' : 'text-slate-100'}`}>{x.name}</span>
                {k?.source && <span className="size-1.5 rounded-full bg-emerald-400 shadow-[0_0_6px_#34d399]" title="Clé configurée" />}
              </div>
              <div className="mt-1 text-[11px] leading-snug text-slate-400">{x.tagline}</div>
            </button>
          )
        })}
      </div>

      <div
        className={`flex items-center gap-2.5 rounded-xl border px-4 py-3 text-[12px] ${
          status?.ready ? 'border-emerald-400/20 bg-emerald-400/[0.06] text-emerald-200' : 'border-amber-400/20 bg-amber-400/[0.06] text-amber-100'
        }`}
      >
        <span className={`size-2 rounded-full ${status?.ready ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]' : 'bg-amber-300'}`} />
        {status?.ready ? (
          <span>
            A.R.I.A est connectée : <span className="font-mono">{status.name}</span>
          </span>
        ) : (
          <span>{status?.error ?? 'Vérification…'}</span>
        )}
      </div>

      <Group title={`Connexion · ${meta.name}`}>
        <Row
          label="Clé API"
          stacked
          help={
            <>
              Rangée dans le {navigator.platform.includes('Mac') ? 'Trousseau macOS' : 'Gestionnaire d’identification Windows'}, jamais en
              clair sur le disque. Obtenir une clé : <span className="font-mono text-slate-300">{meta.keyUrl}</span>
              {p === 'openai' && ' (inutile pour un modèle local comme Ollama).'}
            </>
          }
        >
          <div className="space-y-2">
            {key?.source && (
              <div className="flex items-center justify-between rounded-lg border border-white/[0.06] bg-black/20 px-3 py-2">
                <span className="text-[12px] text-slate-300">
                  {key.source === 'keychain' ? 'Enregistrée dans le trousseau' : 'Lue depuis une variable d’environnement'}
                  {key.hint && <span className="ml-2 font-mono text-slate-500">••••{key.hint}</span>}
                </span>
                {key.source === 'keychain' && (
                  <Button variant="danger" onClick={async () => onStatus(await bridge.setAPIKey(p, ''))}>
                    Supprimer
                  </Button>
                )}
              </div>
            )}
            <SecretField
              placeholder={key?.source ? 'Remplacer la clé…' : 'Colle ta clé ici'}
              onSave={async (v) => onStatus(await bridge.setAPIKey(p, v))}
            />
          </div>
        </Row>
        <Row
          label="Adresse de l’API"
          help={p === 'openai' ? 'Ex. https://openrouter.ai/api/v1 ou http://localhost:11434/v1 (Ollama). Vide : OpenAI.' : 'Vide : adresse officielle. Utile pour un proxy.'}
          stacked
        >
          <input
            className={`${inputClass} font-mono`}
            placeholder="officielle"
            defaultValue={s.ai.baseUrls[p] ?? ''}
            key={p}
            onBlur={(e) => {
              const v = e.target.value.trim()
              update((d) => void (d.ai.baseUrls[p] = v))
            }}
          />
        </Row>
        <Row label="Modèle" help="Saisis un identifiant ou récupère la liste de ton compte." stacked>
          <div className="flex gap-2">
            <input
              key={p}
              list={listId}
              className={`${inputClass} font-mono`}
              defaultValue={model}
              placeholder={p === 'openai' ? 'ex. gpt-…' : DEFAULT_MODELS[p]}
              onBlur={(e) => {
                const v = e.target.value.trim()
                if (v !== model) update((d) => void (d.ai.models[p] = v))
              }}
              onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
            />
            <datalist id={listId}>
              {suggestions.map((m) => (
                <option key={m} value={m} />
              ))}
            </datalist>
            <Button onClick={refreshModels} busy={loadingModels}>
              Lister
            </Button>
          </div>
          {models[p] && <div className="mt-1.5 text-[11px] text-slate-500">{models[p].length} modèles disponibles : clique dans le champ pour les voir.</div>}
          {p === 'gemini' && (
            <div className="mt-1.5 text-[11px] text-slate-500">
              Mesuré sur le niveau gratuit : 3.6 Flash ≈ 2-3 s, 3.5 Flash-Lite ≈ 1 s, 3.8 Flash souvent saturé.
            </div>
          )}
        </Row>
        <Row label="Tester la connexion" help="Envoie une courte question texte et mesure le temps de réponse.">
          <Button variant="primary" onClick={runTest} busy={test.busy}>
            Tester
          </Button>
        </Row>
        {(test.result || test.error) && (
          <div className="px-4 py-3 text-[12px]">
            {test.result ? (
              <div className="space-y-1">
                <div className="font-mono text-[11px] text-emerald-300">
                  ✓ {test.result.model} · premier mot en {(test.result.millis / 1000).toFixed(2)} s
                </div>
                <div className="text-slate-300">« {test.result.reply} »</div>
              </div>
            ) : (
              <div className="text-rose-200">{test.error}</div>
            )}
          </div>
        )}
        {error && <div className="px-4 py-3 text-[12px] text-rose-200">{error}</div>}
      </Group>

      <Group title="Comportement">
        <Row label="Réflexion" help="Plus de réflexion = réponses plus fines mais plus lentes. « Rapide » est recommandé en jeu.">
          <Segmented
            value={s.ai.thinking}
            options={[
              {value: 'low', label: 'Rapide'},
              {value: 'medium', label: 'Équilibré'},
              {value: 'high', label: 'Approfondi'},
            ]}
            onChange={(v) => update((d) => void (d.ai.thinking = v))}
          />
        </Row>
        <Row label="Longueur des réponses">
          <Segmented
            value={s.ai.length}
            options={[
              {value: 'short', label: 'Courte'},
              {value: 'normal', label: 'Normale'},
              {value: 'detailed', label: 'Détaillée'},
            ]}
            onChange={(v) => update((d) => void (d.ai.length = v))}
          />
        </Row>
        <Row label="Langue d’A.R.I.A">
          <Segmented
            value={s.ai.language}
            options={[
              {value: 'fr', label: 'Français'},
              {value: 'en', label: 'English'},
            ]}
            onChange={(v) => update((d) => void (d.ai.language = v))}
          />
        </Row>
        <Row label="Mémoire de la conversation" help="Échanges précédents envoyés comme contexte (en texte, sans les captures).">
          <Slider
            label="Mémoire"
            value={s.ai.historyTurns}
            min={0}
            max={20}
            onChange={(v) => update((d) => void (d.ai.historyTurns = v))}
            format={(v) => (v === 0 ? 'aucune' : `${v} éch.`)}
          />
        </Row>
        <Row label="Instructions personnalisées" help="Ajoutées à la personnalité d’A.R.I.A. Ex. « Je joue support sur League of Legends. »" stacked>
          <textarea
            rows={3}
            maxLength={2000}
            defaultValue={s.ai.customInstructions}
            onBlur={(e) => update((d) => void (d.ai.customInstructions = e.target.value))}
            className={`${inputClass} resize-none`}
            placeholder="Optionnel"
          />
        </Row>
        <Row
          label="Repli de sécurité"
          help="Claude uniquement : si un filtre de sécurité refuse une demande, elle est relancée sur un autre modèle Claude."
          disabled={p !== 'claude'}
        >
          <Toggle label="Repli de sécurité" checked={s.ai.fallback} onChange={(v) => update((d) => void (d.ai.fallback = v))} />
        </Row>
      </Group>
    </div>
  )
}

// ---- Vision --------------------------------------------------------------------

export function VisionSection({s, update}: SectionProps) {
  return (
    <div className="space-y-6">
      <Group title="Capture d’écran">
        <Row label="A.R.I.A voit l’écran par défaut" help="Chaque question part avec une capture prise à l’instant. L’œil du chat la coupe ponctuellement.">
          <Toggle label="Vision par défaut" checked={s.vision.enabled} onChange={(v) => update((d) => void (d.vision.enabled = v))} />
        </Row>
        <Row label="Résolution" help="Côté le plus long. Au-delà de 1568 px, la plupart des modèles réduisent l’image eux-mêmes.">
          <Segmented
            value={s.vision.maxSide}
            options={[
              {value: 1024, label: '1024'},
              {value: 1568, label: '1568'},
              {value: 2048, label: '2048'},
            ]}
            onChange={(v) => update((d) => void (d.vision.maxSide = v))}
          />
        </Row>
        <Row label="Qualité JPEG" help="Plus haut = texte plus net, envoi plus lourd.">
          <Slider label="Qualité" value={s.vision.quality} min={40} max={95} onChange={(v) => update((d) => void (d.vision.quality = v))} format={(v) => `${v} %`} />
        </Row>
        <Row label="Inclure le curseur" help="Utile pour montrer à A.R.I.A de quoi tu parles.">
          <Toggle label="Curseur" checked={s.vision.showCursor} onChange={(v) => update((d) => void (d.vision.showCursor = v))} />
        </Row>
      </Group>
      <p className="px-1 text-[11.5px] leading-relaxed text-slate-500">
        Spark est toujours exclu de ses propres captures : A.R.I.A voit ce qu’il y a derrière le chat, jamais le chat lui-même.
      </p>
    </div>
  )
}

// ---- Voix ----------------------------------------------------------------------

export function VoiceSection({s, update}: SectionProps) {
  const [voices, setVoices] = useState<Voice[]>([])
  useEffect(() => {
    void bridge.listVoices().then(setVoices).catch(() => setVoices([]))
  }, [])

  const lang = s.ai.language
  const sorted = [...voices].sort((a, b) => {
    const pa = a.lang.startsWith(lang) ? 0 : 1
    const pb = b.lang.startsWith(lang) ? 0 : 1
    return pa - pb || a.name.localeCompare(b.name)
  })

  return (
    <div className="space-y-6">
      <Group title="Voix d’A.R.I.A">
        <Row label="Voix activée">
          <Toggle label="Voix" checked={s.voice.enabled} onChange={(v) => update((d) => void (d.voice.enabled = v))} />
        </Row>
        <Row label="Voix" help="« Automatique » choisit la meilleure voix féminine française installée." disabled={!s.voice.enabled}>
          <Select
            label="Voix"
            value={s.voice.voice}
            onChange={(v) => update((d) => void (d.voice.voice = v))}
            options={[{value: '', label: 'Automatique'}, ...sorted.map((v) => ({value: v.name, label: `${v.name} · ${v.lang}`}))]}
          />
        </Row>
        <Row label="Vitesse" disabled={!s.voice.enabled}>
          <Slider
            label="Vitesse"
            value={s.voice.rate}
            min={0.5}
            max={2}
            step={0.05}
            onChange={(v) => update((d) => void (d.voice.rate = v))}
            format={(v) => `×${v.toFixed(2)}`}
          />
        </Row>
        <Row label="Lire les réponses à voix haute" disabled={!s.voice.enabled}>
          <Toggle label="Lire les réponses" checked={s.voice.readAnswers} onChange={(v) => update((d) => void (d.voice.readAnswers = v))} />
        </Row>
        <Row label="Message d’accueil au démarrage">
          <Toggle label="Accueil" checked={s.voice.greeting} onChange={(v) => update((d) => void (d.voice.greeting = v))} />
        </Row>
        <Row label="Écouter un essai" disabled={!s.voice.enabled}>
          <Button variant="primary" onClick={bridge.testVoice}>
            ▶ Tester la voix
          </Button>
        </Row>
      </Group>
      <p className="px-1 text-[11.5px] leading-relaxed text-slate-500">
        Pour une voix bien plus naturelle sur macOS : Réglages Système › Accessibilité › Contenu énoncé › Voix système › Gérer les voix ›
        Français › <span className="text-slate-300">Audrey (Premium)</span>. Une voix neuronale arrivera avec le module audio.
      </p>
    </div>
  )
}

// ---- Micro -----------------------------------------------------------------------

function MicMeter({sensitivity, continuous}: {sensitivity: number; continuous: boolean}) {
  const bar = useRef<HTMLDivElement>(null)
  const [on, setOn] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!on) return
    // Level events update the bar directly (no React render at 20 Hz).
    const off = bridge.onVoiceLevel((l) => {
      if (bar.current) bar.current.style.width = `${Math.round(l * 100)}%`
    })
    return () => {
      off()
      void bridge.setMicTest(false)
    }
  }, [on])

  async function toggle() {
    setError(null)
    if (on) return setOn(false)
    try {
      await bridge.setMicTest(true)
      setOn(true)
    } catch (e) {
      setError(errorText(e))
    }
  }

  // Same mapping as voice.threshold() in Go, on the 0-1 scale of the meter.
  const threshold = Math.min(1, (0.004 + 0.05 * (1 - sensitivity / 100)) * 8)

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-3">
        <Button variant={on ? 'danger' : 'primary'} onClick={toggle}>
          {on ? 'Arrêter' : 'Tester le micro'}
        </Button>
        <div className="relative h-2.5 flex-1 overflow-hidden rounded-full bg-white/[0.06]">
          <div ref={bar} className="h-full w-0 rounded-full bg-gradient-to-r from-spark-teal to-spark-cyan transition-[width] duration-75" />
          {continuous && (
            <span
              className="absolute top-[-3px] bottom-[-3px] w-[2px] rounded bg-amber-300"
              style={{left: `${threshold * 100}%`}}
              title="Seuil de détection de la voix"
            />
          )}
        </div>
      </div>
      {continuous && on && <div className="text-[11px] text-slate-500">Le repère jaune est le seuil : ta voix doit le dépasser, le bruit de fond non.</div>}
      {error && <div className="text-[11.5px] text-rose-200">{error}</div>}
    </div>
  )
}

export function MicSection({s, update}: SectionProps) {
  const [status, setStatus] = useState<MicStatus | null>(null)
  const [asking, setAsking] = useState(false)
  useEffect(() => {
    void bridge.getMicStatus().then(setStatus)
  }, [])

  const granted = status?.mic === 'granted' && status?.speech === 'granted'
  const denied = status?.mic === 'denied' || status?.speech === 'denied'
  const ptt = s.mic.mode === 'ptt'
  const continuous = s.mic.mode === 'continuous'

  return (
    <div className="space-y-6">
      <div
        className={`flex items-center justify-between gap-4 rounded-xl border px-4 py-3 text-[12px] ${
          granted ? 'border-emerald-400/20 bg-emerald-400/[0.06] text-emerald-200' : 'border-amber-400/20 bg-amber-400/[0.06] text-amber-100'
        }`}
      >
        <span className="flex items-center gap-2.5">
          <span className={`size-2 rounded-full ${granted ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]' : 'bg-amber-300'}`} />
          {granted
            ? 'Micro et reconnaissance vocale autorisés.'
            : denied
              ? 'Accès refusé : autorise Spark dans Réglages Système › Confidentialité › Micro et Reconnaissance vocale.'
              : 'Spark a besoin du micro et de la reconnaissance vocale.'}
        </span>
        {!granted && !denied && (
          <Button
            variant="primary"
            busy={asking}
            onClick={async () => {
              setAsking(true)
              try {
                setStatus(await bridge.requestMicPermission())
              } finally {
                setAsking(false)
              }
            }}
          >
            Autoriser
          </Button>
        )}
      </div>

      <Group title="Parler à A.R.I.A">
        <Row label="Mode d’écoute" help="Push-to-talk : maintiens la touche, parle, relâche. Continu : A.R.I.A détecte ta voix toute seule.">
          <Segmented
            value={s.mic.mode}
            options={[
              {value: 'off', label: 'Désactivé'},
              {value: 'ptt', label: 'Push-to-talk'},
              {value: 'continuous', label: 'Continu'},
            ]}
            onChange={(v) => update((d) => void (d.mic.mode = v))}
          />
        </Row>
        <Row
          label="Touche push-to-talk"
          help="Une touche F seule est idéale en jeu. La capture d’écran est prise à l’appui ; appuyer coupe aussi A.R.I.A si elle parle."
          disabled={!ptt}
        >
          <ShortcutRecorder value={s.mic.pushToTalk} requireModifier={false} onChange={(v) => update((d) => void (d.mic.pushToTalk = v))} />
        </Row>
        <Row label="Sons de retour" help="Un léger son quand l’écoute commence et s’arrête." disabled={s.mic.mode === 'off'}>
          <Toggle label="Sons" checked={s.mic.cues} onChange={(v) => update((d) => void (d.mic.cues = v))} />
        </Row>
      </Group>

      <Group title="Écoute continue">
        <Row
          label="Mot d’activation"
          help="A.R.I.A ne répond qu’aux phrases qui commencent par ce mot : « Aria, que dois-je faire ? ». Sans lui, toute parole est envoyée."
          disabled={!continuous}
        >
          <Toggle label="Mot d’activation" checked={s.mic.wakeWord} onChange={(v) => update((d) => void (d.mic.wakeWord = v))} />
        </Row>
        <Row label="Mot" disabled={!continuous || !s.mic.wakeWord}>
          <input
            key={s.mic.wakePhrase}
            defaultValue={s.mic.wakePhrase}
            maxLength={40}
            onBlur={(e) => {
              const v = e.target.value.trim()
              if (v) update((d) => void (d.mic.wakePhrase = v))
            }}
            onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
            className={`${inputClass} w-44`}
          />
        </Row>
        <Row label="Sensibilité" help="Plus haut = A.R.I.A se déclenche sur une voix plus faible (et sur plus de bruit)." disabled={!continuous}>
          <Slider label="Sensibilité" value={s.mic.sensitivity} min={0} max={100} onChange={(v) => update((d) => void (d.mic.sensitivity = v))} format={(v) => `${v} %`} />
        </Row>
      </Group>

      <Group title="Test et reconnaissance">
        <Row label="Niveau du micro" stacked>
          <MicMeter sensitivity={s.mic.sensitivity} continuous={continuous} />
        </Row>
        <Row label="Reconnaissance vocale" help={status?.live ? 'Transcription en direct, ta voix ne quitte pas l’ordinateur quand le modèle local est disponible.' : 'L’enregistrement est transcrit à la fin de la phrase.'}>
          <span className="max-w-64 text-right text-[12px] text-slate-300">{status?.engine ?? '…'}</span>
        </Row>
      </Group>
    </div>
  )
}

// ---- Raccourcis --------------------------------------------------------------------

export function HotkeysSection({s, update, hotkeyError}: SectionProps & {hotkeyError?: string}) {
  const rows = [
    {k: 'toggleChat', label: 'Ouvrir / fermer A.R.I.A', help: 'Prend le focus clavier, le rend à l’app précédente à la fermeture.'},
    {k: 'toggleVisible', label: 'Masquer / afficher Spark', help: 'Masqué, Spark laisse passer tous les clics et ne consomme plus rien.'},
    {k: 'openSettings', label: 'Ouvrir les paramètres', help: undefined},
  ] as const
  return (
    <div className="space-y-6">
      <Group title="Raccourcis globaux">
        {rows.map((r) => (
          <Row key={r.k} label={r.label} help={r.help}>
            <ShortcutRecorder value={s.hotkeys[r.k]} onChange={(v) => update((d) => void (d.hotkeys[r.k] = v))} />
          </Row>
        ))}
      </Group>
      {hotkeyError && (
        <div className="rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-[12px] text-rose-200">
          {hotkeyError}. Choisis une autre combinaison.
        </div>
      )}
      <p className="px-1 text-[11.5px] leading-relaxed text-slate-500">
        Les raccourcis fonctionnent même quand un jeu ou une autre app est au premier plan. Clique sur un raccourci puis appuie sur la
        nouvelle combinaison.
      </p>
    </div>
  )
}

// ---- Apparence -----------------------------------------------------------------------

export function AppearanceSection({s, update, onResetOrb}: SectionProps & {onResetOrb: () => void}) {
  return (
    <div className="space-y-6">
      <Group title="Thème">
        <Row label="Couleur d’accent">
          <div className="flex gap-2">
            {(Object.keys(ACCENTS) as Accent[]).map((a) => (
              <button
                key={a}
                type="button"
                title={ACCENTS[a].label}
                aria-label={ACCENTS[a].label}
                onClick={() => update((d) => void (d.appearance.accent = a))}
                className={`size-7 rounded-full transition-transform hover:scale-110 ${
                  s.appearance.accent === a ? 'ring-2 ring-white/80 ring-offset-2 ring-offset-slate-900' : ''
                }`}
                style={{background: `linear-gradient(135deg, ${ACCENTS[a].light}, ${ACCENTS[a].main} 50%, ${ACCENTS[a].second})`}}
              />
            ))}
          </div>
        </Row>
        <Row label="Opacité des panneaux" help="Plus bas = on voit davantage le jeu derrière.">
          <Slider
            label="Opacité"
            value={s.appearance.panelOpacity}
            min={50}
            max={100}
            onChange={(v) => update((d) => void (d.appearance.panelOpacity = v))}
            format={(v) => `${v} %`}
          />
        </Row>
        <Row label="Réduire les animations">
          <Toggle label="Animations réduites" checked={s.appearance.reduceMotion} onChange={(v) => update((d) => void (d.appearance.reduceMotion = v))} />
        </Row>
      </Group>
      <Group title="Bouton A.R.I.A">
        <Row label="Taille">
          <Segmented
            value={s.appearance.orbSize}
            options={[
              {value: 'small', label: 'Petit'},
              {value: 'medium', label: 'Moyen'},
              {value: 'large', label: 'Grand'},
            ]}
            onChange={(v) => update((d) => void (d.appearance.orbSize = v))}
          />
        </Row>
        <Row label="Bulles de message" help="Affiche ce que dit A.R.I.A à côté du bouton quand le chat est fermé.">
          <Toggle label="Bulles" checked={s.appearance.showBubbles} onChange={(v) => update((d) => void (d.appearance.showBubbles = v))} />
        </Row>
        <Row label="Position" help="Tu peux aussi glisser le bouton n’importe où.">
          <Button onClick={onResetOrb}>Remettre à sa place</Button>
        </Row>
      </Group>
    </div>
  )
}

// ---- Confidentialité ---------------------------------------------------------------------

export function PrivacySection({s, update, onCleared}: SectionProps & {onCleared: () => void}) {
  const [cleared, setCleared] = useState(false)
  return (
    <div className="space-y-6">
      <Group title="Ce qui quitte ton ordinateur">
        <div className="space-y-2 px-4 py-3.5 text-[12px] leading-relaxed text-slate-300">
          <p>
            À chaque question : ton texte, la capture d’écran (si la vision est active) et les {s.ai.historyTurns} derniers échanges en
            texte, envoyés au fournisseur choisi. Rien d’autre, et rien entre deux questions.
          </p>
          <p className="text-amber-200/90">
            Sur le niveau gratuit de Gemini, Google peut utiliser ces requêtes pour améliorer ses produits. Coupe la vision sur les écrans
            sensibles (mails, banque, mots de passe).
          </p>
        </div>
        <Row label="Vision par défaut">
          <Toggle label="Vision par défaut" checked={s.vision.enabled} onChange={(v) => update((d) => void (d.vision.enabled = v))} />
        </Row>
      </Group>
      <Group title="Données locales">
        <Row label="Effacer la mémoire de la conversation" help="A.R.I.A oublie le contexte des échanges précédents.">
          <Button
            variant="danger"
            onClick={() => {
              bridge.clearConversation()
              onCleared()
              setCleared(true)
            }}
          >
            {cleared ? 'Effacée ✓' : 'Effacer'}
          </Button>
        </Row>
        <div className="px-4 py-3.5 text-[11.5px] leading-relaxed text-slate-400">
          Les clés API sont dans le trousseau du système. Les paramètres sont dans un fichier local (voir « À propos »). Aucune
          télémétrie.
        </div>
      </Group>
    </div>
  )
}

// ---- Avancé ---------------------------------------------------------------------------------

export function AdvancedSection({s, update, onReset}: SectionProps & {onReset: () => void}) {
  const [confirm, setConfirm] = useState(false)
  return (
    <div className="space-y-6">
      <Group title="Performance">
        <Row
          label="Détection du survol"
          help="Fréquence de lecture du curseur pour le click-through. 30 Hz économise le CPU, 120 Hz rend les boutons plus réactifs."
        >
          <Segmented
            value={s.advanced.pollHz}
            options={[
              {value: 30, label: '30 Hz'},
              {value: 60, label: '60 Hz'},
              {value: 120, label: '120 Hz'},
            ]}
            onChange={(v) => update((d) => void (d.advanced.pollHz = v))}
          />
        </Row>
      </Group>
      <Group title="Développement">
        <Row label="Mode debug" help="Journal détaillé dans le terminal et zones cliquables entourées en jaune.">
          <Toggle label="Debug" checked={s.advanced.debug} onChange={(v) => update((d) => void (d.advanced.debug = v))} />
        </Row>
      </Group>
      <Group title="Réinitialisation">
        <Row label="Rétablir tous les paramètres par défaut" help="Les clés API et la position du bouton sont conservées.">
          <Button
            variant="danger"
            onClick={() => {
              if (!confirm) return setConfirm(true)
              setConfirm(false)
              onReset()
            }}
          >
            {confirm ? 'Confirmer ?' : 'Réinitialiser'}
          </Button>
        </Row>
      </Group>
    </div>
  )
}

// ---- À propos -------------------------------------------------------------------------------

export function AboutSection() {
  const [info, setInfo] = useState<AppInfo | null>(null)
  useEffect(() => {
    void bridge.getAppInfo().then(setInfo)
  }, [])
  return (
    <div className="space-y-6">
      <Group>
        <Row label="Version">
          <span className="font-mono text-[12px] text-slate-300">{info?.version}</span>
        </Row>
        <Row label="Plateforme">
          <span className="font-mono text-[12px] text-slate-300">{info?.platform}</span>
        </Row>
        <Row label="Fichier de paramètres" stacked>
          <code className="block rounded-lg bg-black/30 px-3 py-2 font-mono text-[11px] break-all text-slate-300 select-text">
            {info?.settingsPath || 'en mémoire'}
          </code>
        </Row>
        <Row label="Code source">
          <span className="font-mono text-[12px] text-slate-300">github.com/Yoyotaa/Spark</span>
        </Row>
      </Group>
    </div>
  )
}
