import {useCallback, useEffect, useLayoutEffect, useRef, useState} from 'react'
import {Bubble, type BubbleContent} from './components/Bubble'
import {ChatPanel, type Message} from './components/ChatPanel'
import {Guard} from './components/Guard'
import {Orb} from './components/Orb'
import {SettingsPanel} from './components/settings/SettingsPanel'
import {useDrag} from './hooks/useDrag'
import {useSettings} from './hooks/useSettings'
import * as bridge from './lib/bridge'
import type {Point, VoiceState} from './lib/bridge'
import {ORB_SIZES, shortcutLabel} from './lib/settings'
import {startRegionTracking} from './overlay/regions'

const MARGIN = 16
const GAP = 14
const PANEL = {w: 352, h: 480} // w-[22rem] h-[30rem]
const BUBBLE_W = 320 // w-80
const BUBBLE_LINGER_MS = 3500

const clamp = (p: Point, size: number): Point => ({
  x: Math.min(Math.max(p.x, MARGIN), window.innerWidth - size - MARGIN),
  y: Math.min(Math.max(p.y, MARGIN), window.innerHeight - size - MARGIN),
})

// Right edge, a bit above the middle: away from the Dock and hot corners.
const defaultPosition = (size: number): Point =>
  clamp({x: window.innerWidth - size - 32, y: Math.round(window.innerHeight * 0.36)}, size)

/** Opens the panel on the side of the screen that has room. */
function placement(p: Point, size: number) {
  const side: 'left' | 'right' = p.x + size / 2 > window.innerWidth / 2 ? 'left' : 'right'
  const top = Math.min(
    Math.max(p.y + size / 2 > window.innerHeight / 2 ? p.y + size - PANEL.h : p.y, MARGIN),
    window.innerHeight - PANEL.h - MARGIN,
  )
  const dx = (w: number) => (side === 'left' ? -GAP - w : size + GAP)
  return {side, panel: {left: dx(PANEL.w), top: top - p.y}, bubbleLeft: dx(BUBBLE_W)}
}

let nextId = 1

export default function App() {
  const {settings, update, saveState, saveError, hotkeyError, reset} = useSettings()
  const size = ORB_SIZES[settings?.appearance.orbSize ?? 'medium']
  const sizeRef = useRef(size)
  sizeRef.current = size

  const [pos, setPos] = useState<Point | null>(null)
  const savedPos = useRef(false)
  const anchor = useRef<HTMLDivElement>(null)

  const [visible, setVisible] = useState(true)
  const [chatOpen, setChatOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [permissionPending, setPermissionPending] = useState(false)
  const [speaking, setSpeaking] = useState(false)
  const [messages, setMessages] = useState<Message[]>([])
  const [waiting, setWaiting] = useState(false)
  const [seeScreen, setSeeScreen] = useState(true)
  const [aiReady, setAiReady] = useState(false)
  const [envDebug, setEnvDebug] = useState(false)
  const [bubble, setBubble] = useState<(BubbleContent & {key: number}) | null>(null)
  const [voiceState, setVoiceState] = useState<VoiceState>('idle')
  const [partial, setPartial] = useState('')

  const chatOpenRef = useRef(chatOpen)
  chatOpenRef.current = chatOpen

  useEffect(() => startRegionTracking(), [])

  useEffect(() => {
    void bridge.isDebug().then(setEnvDebug)
  }, [])
  const debug = envDebug || !!settings?.advanced.debug
  useEffect(() => {
    document.body.classList.toggle('debug', debug)
  }, [debug])

  // Settings loaded: restore the orb, then start the welcome sequence.
  const loaded = settings !== null
  useEffect(() => {
    if (!settings) return
    savedPos.current = !!settings.orb
    setPos(settings.orb ? clamp(settings.orb, size) : defaultPosition(size))
    bridge.boot()
    // Only once, when the settings arrive.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loaded])

  // The default vision state follows the setting.
  const visionDefault = settings?.vision.enabled ?? true
  useEffect(() => setSeeScreen(visionDefault), [visionDefault])

  // Keep the orb on screen when its size or the screen changes.
  useEffect(() => {
    setPos((p) => (p ? clamp(p, size) : p))
  }, [size])
  useEffect(() => {
    const onResize = () =>
      setPos((p) => (p && savedPos.current ? clamp(p, sizeRef.current) : defaultPosition(sizeRef.current)))
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  // AI connection state, refreshed when the settings close.
  useEffect(() => {
    if (!settingsOpen) void bridge.getAIStatus().then((s) => setAiReady(s.ready))
  }, [settingsOpen])

  // First launch (no AI configured): open the settings on the AI section,
  // with a welcome, rather than letting questions fail.
  const onboarded = useRef(false)
  useEffect(() => {
    if (!loaded || onboarded.current) return
    onboarded.current = true
    void bridge.getAIStatus().then((s) => {
      if (!s.ready) setSettingsOpen(true)
    })
  }, [loaded])

  useEffect(() => bridge.onToggleChat(() => setChatOpen((o) => !o)), [])
  useEffect(() => bridge.onOpenSettings(() => setSettingsOpen(true)), [])
  useEffect(() => bridge.onPermission(setPermissionPending), [])
  useEffect(
    () =>
      bridge.onVisibility((v) => {
        setVisible(v)
        if (!v) {
          setChatOpen(false)
          setSettingsOpen(false)
        }
      }),
    [],
  )
  const showBubbles = settings?.appearance.showBubbles ?? true
  const showBubblesRef = useRef(showBubbles)
  showBubblesRef.current = showBubbles
  useEffect(
    () =>
      bridge.onSay((text) => {
        setMessages((m) => [...m, {id: `say-${nextId++}`, from: 'aria', text, at: new Date()}])
        if (!chatOpenRef.current && showBubblesRef.current) setBubble({kind: 'say', text, key: nextId++})
      }),
    [],
  )
  useEffect(() => bridge.onSpeaking(setSpeaking), [])

  // A newer version exists: say it once, quietly (not read aloud).
  useEffect(
    () =>
      bridge.onUpdate((r) => {
        if (showBubblesRef.current)
          setBubble({kind: 'say', text: `Spark ${r.version} est disponible : Paramètres › À propos.`, key: nextId++})
      }),
    [],
  )

  // ---- Voice -----------------------------------------------------------------
  const bubbleOn = () => !chatOpenRef.current && showBubblesRef.current

  useEffect(
    () =>
      bridge.onVoiceState((st) => {
        setVoiceState(st)
        if (st === 'listening') {
          setPartial('')
          if (bubbleOn()) setBubble({kind: 'listen', text: '', transcribing: false, key: nextId++})
        } else if (st === 'transcribing') {
          setBubble((b) => (b?.kind === 'listen' ? {...b, transcribing: true} : b))
        } else {
          // Idle without a turn (nothing heard, no wake phrase): drop the
          // listening bubble; a turn replaces it with the answer.
          setBubble((b) => (b?.kind === 'listen' ? null : b))
        }
      }),
    [],
  )
  useEffect(
    () =>
      bridge.onVoicePartial((text) => {
        setPartial(text)
        setBubble((b) => (b?.kind === 'listen' ? {...b, text} : b))
      }),
    [],
  )
  useEffect(
    () =>
      bridge.onVoiceTurn(({text, turn, source}) => {
        setPartial('')
        setMessages((m) => [...m, {id: `user-${nextId++}`, from: 'user', text, at: new Date(), image: turn.capture, voice: source !== 'quick'}])
        setWaiting(true)
        if (bubbleOn()) setBubble({kind: 'answer', turnId: turn.id, question: text, text: '', done: false, key: nextId++})
        // (voice:turn now precedes the answer; the chat message keyed
        // turn-<id> is created by the first delta.)
      }),
    [],
  )
  useEffect(
    () =>
      bridge.onVoiceError((message) => {
        setPartial('')
        setMessages((m) => [...m, {id: `err-${nextId++}`, from: 'aria', text: message, at: new Date(), error: true}])
        if (bubbleOn()) setBubble({kind: 'error', text: message, key: nextId++})
      }),
    [],
  )
  // The mic level drives a CSS variable directly: no React render at 20 Hz.
  useEffect(
    () =>
      bridge.onVoiceLevel((level) => {
        anchor.current?.style.setProperty('--level', level.toFixed(3))
      }),
    [],
  )

  // Streamed answers: one message per turn, created by its first delta.
  useEffect(
    () =>
      bridge.onDelta(({id, text}) => {
        setWaiting(false)
        setBubble((b) => (b?.kind === 'answer' && b.turnId === id ? {...b, text: b.text + text} : b))
        setMessages((m) => {
          const key = `turn-${id}`
          const i = m.findIndex((x) => x.id === key)
          if (i < 0) return [...m, {id: key, from: 'aria', text, at: new Date(), streaming: true}]
          const copy = m.slice()
          copy[i] = {...copy[i], text: copy[i].text + text}
          return copy
        })
      }),
    [],
  )
  useEffect(
    () =>
      bridge.onDone(({id, error, interrupted}) => {
        setBubble((b) => {
          if (b?.kind !== 'answer' || b.turnId !== id) return b
          if (error) return {kind: 'error', text: error, key: b.key}
          return {...b, done: true, text: b.text + (interrupted ? '…' : '')}
        })
        setMessages((m) => {
          const key = `turn-${id}`
          const i = m.findIndex((x) => x.id === key)
          if (error) {
            const msg: Message = {id: key, from: 'aria', text: error, at: new Date(), error: true}
            return i < 0 ? [...m, msg] : m.map((x) => (x.id === key ? msg : x))
          }
          if (i < 0) return m
          const copy = m.slice()
          copy[i] = {...copy[i], streaming: false, text: copy[i].text + (interrupted ? '…' : '')}
          return copy
        })
        if (error) setWaiting(false)
      }),
    [],
  )

  // The bubble lingers a little once A.R.I.A stops talking (longer for an
  // answer, to finish reading it).
  const bubbleBusy = bubble?.kind === 'listen' || (bubble?.kind === 'answer' && !bubble.done)
  useEffect(() => {
    if (!bubble || speaking || bubbleBusy) return
    const ms = bubble.kind === 'answer' ? BUBBLE_LINGER_MS + bubble.text.length * 40 : BUBBLE_LINGER_MS
    const id = setTimeout(() => setBubble(null), ms)
    return () => clearTimeout(id)
  }, [bubble, speaking, bubbleBusy])

  // Keyboard focus while the chat or the settings are open, handed back to
  // the previous app afterwards. Compares with the last notified value:
  // StrictMode runs effects twice.
  const focused = chatOpen || settingsOpen
  const notified = useRef(focused)
  useEffect(() => {
    if (chatOpen) setBubble(null)
  }, [chatOpen])
  useEffect(() => {
    if (notified.current === focused) return
    notified.current = focused
    bridge.setKeyboardFocus(focused)
  }, [focused])

  const send = useCallback(
    async (text: string) => {
      const userId = `user-${nextId++}`
      setMessages((m) => [...m, {id: userId, from: 'user', text, at: new Date()}])
      setWaiting(true)
      try {
        const turn = await bridge.ask(text, seeScreen)
        // Superseded by a newer question (push-to-talk...): nothing to wait for.
        if (turn.interrupted) setWaiting(false)
        if (turn.capture) {
          setMessages((m) => m.map((x) => (x.id === userId ? {...x, image: turn.capture} : x)))
        }
      } catch (e) {
        setWaiting(false)
        const error = e instanceof Error ? e.message : String(e)
        setMessages((m) => [...m, {id: `err-${nextId++}`, from: 'aria', text: error, at: new Date(), error: true}])
      }
    },
    [seeScreen],
  )

  // Dragging moves the anchor directly (no React render per frame); the
  // state is committed once, on drop.
  const drag = useDrag({
    getOrigin: () => pos ?? defaultPosition(size),
    clamp: (p) => clamp(p, size),
    onMove: (p) => {
      if (anchor.current) anchor.current.style.transform = `translate3d(${p.x}px, ${p.y}px, 0)`
    },
    onDrop: (p) => {
      savedPos.current = true
      setPos(p)
      bridge.saveOrbPosition(p)
    },
    onTap: () => setChatOpen((o) => !o),
  })

  useLayoutEffect(() => {
    if (anchor.current && pos) anchor.current.style.transform = `translate3d(${pos.x}px, ${pos.y}px, 0)`
  }, [pos])

  if (!pos || !settings) return null
  const place = placement(pos, size)

  // Everything outside [data-interactive] elements is click-through.
  return (
    <main className="pointer-events-none fixed inset-0" hidden={!visible}>
      {/* macOS permission dialog pending: everything steps aside (the
          window is click-through) so the dialog stays visible and
          clickable even below the overlay. */}
      {permissionPending && (
        <div className="fixed top-6 left-1/2 flex -translate-x-1/2 animate-panel-in items-center gap-3 rounded-xl border border-spark-cyan/30 bg-panel px-4 py-2.5 text-[12.5px] text-slate-100 shadow-2xl backdrop-blur-xl">
          <span className="size-2 animate-pulse rounded-full bg-spark-cyan" />
          Réponds à la fenêtre de macOS pour autoriser Spark (micro, reconnaissance vocale ou capture d’écran).
        </div>
      )}
      <div hidden={permissionPending}>
      <div ref={anchor} className="pointer-events-auto absolute top-0 left-0 will-change-transform">
        <Orb size={size} open={chatOpen} speaking={speaking} listening={voiceState === 'listening'} handlers={drag} />

        <div className="absolute" style={place.panel}>
          <Guard name="chat">
          <ChatPanel
            open={chatOpen}
            speaking={speaking}
            waiting={waiting}
            aiReady={aiReady}
            seeScreen={seeScreen}
            listening={voiceState}
            partial={partial}
            messages={messages}
            onSend={send}
            onToggleScreen={() => setSeeScreen((v) => !v)}
            onOpenSettings={() => setSettingsOpen(true)}
            onStop={bridge.interrupt}
            onClose={() => setChatOpen(false)}
            toggleLabel={shortcutLabel(settings.hotkeys.toggleChat)}
          />
          </Guard>
        </div>

        {bubble && !chatOpen && (
          <div className="absolute top-1/2 -translate-y-1/2" style={{left: place.bubbleLeft}}>
            <Guard name="bubble" key={bubble.key}>
              <Bubble content={bubble} speaking={speaking} side={place.side} />
            </Guard>
          </div>
        )}
      </div>

      {settingsOpen && (
        <Guard name="settings">
        <SettingsPanel
          settings={settings}
          update={update}
          saveState={saveState}
          saveError={saveError}
          hotkeyError={hotkeyError}
          onReset={reset}
          onResetOrb={() => {
            const p = defaultPosition(size)
            savedPos.current = false
            setPos(p)
            bridge.saveOrbPosition(p)
          }}
          onCleared={() => setMessages([])}
          onClose={() => setSettingsOpen(false)}
        />
        </Guard>
      )}
      </div>
    </main>
  )
}
