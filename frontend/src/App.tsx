import {useCallback, useEffect, useLayoutEffect, useRef, useState} from 'react'
import {Bubble} from './components/Bubble'
import {ChatPanel, type Message} from './components/ChatPanel'
import {Orb} from './components/Orb'
import {SettingsPanel} from './components/settings/SettingsPanel'
import {useDrag} from './hooks/useDrag'
import {useSettings} from './hooks/useSettings'
import * as bridge from './lib/bridge'
import type {Point} from './lib/bridge'
import {ORB_SIZES, shortcutLabel} from './lib/settings'
import {startRegionTracking} from './overlay/regions'

const MARGIN = 16
const GAP = 14
const PANEL = {w: 352, h: 480} // w-[22rem] h-[30rem]
const BUBBLE_W = 288 // w-72
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
  const [speaking, setSpeaking] = useState(false)
  const [messages, setMessages] = useState<Message[]>([])
  const [waiting, setWaiting] = useState(false)
  const [seeScreen, setSeeScreen] = useState(true)
  const [aiReady, setAiReady] = useState(false)
  const [envDebug, setEnvDebug] = useState(false)
  const [bubble, setBubble] = useState<{text: string; key: number} | null>(null)

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

  useEffect(() => bridge.onToggleChat(() => setChatOpen((o) => !o)), [])
  useEffect(() => bridge.onOpenSettings(() => setSettingsOpen(true)), [])
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
        if (!chatOpenRef.current && showBubblesRef.current) setBubble({text, key: nextId++})
      }),
    [],
  )
  useEffect(() => bridge.onSpeaking(setSpeaking), [])

  // Streamed answers: one message per turn, created by its first delta.
  useEffect(
    () =>
      bridge.onDelta(({id, text}) => {
        setWaiting(false)
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

  // The bubble lingers a little once A.R.I.A stops talking.
  useEffect(() => {
    if (!bubble || speaking) return
    const id = setTimeout(() => setBubble(null), BUBBLE_LINGER_MS)
    return () => clearTimeout(id)
  }, [bubble, speaking])

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
      <div ref={anchor} className="pointer-events-auto absolute top-0 left-0 will-change-transform">
        <Orb size={size} open={chatOpen} speaking={speaking} handlers={drag} />

        <div className="absolute" style={place.panel}>
          <ChatPanel
            open={chatOpen}
            speaking={speaking}
            waiting={waiting}
            aiReady={aiReady}
            seeScreen={seeScreen}
            messages={messages}
            onSend={send}
            onToggleScreen={() => setSeeScreen((v) => !v)}
            onOpenSettings={() => setSettingsOpen(true)}
            onClose={() => setChatOpen(false)}
            toggleLabel={shortcutLabel(settings.hotkeys.toggleChat)}
          />
        </div>

        {bubble && !chatOpen && (
          <div className="absolute top-1/2 -translate-y-1/2" style={{left: place.bubbleLeft}}>
            <Bubble key={bubble.key} text={bubble.text} speaking={speaking} side={place.side} />
          </div>
        )}
      </div>

      {settingsOpen && (
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
      )}
    </main>
  )
}
