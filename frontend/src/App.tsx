import {useCallback, useEffect, useLayoutEffect, useRef, useState} from 'react'
import {Bubble} from './components/Bubble'
import {ChatPanel, type Message} from './components/ChatPanel'
import {ORB_SIZE, Orb} from './components/Orb'
import {useDrag} from './hooks/useDrag'
import * as bridge from './lib/bridge'
import type {Point} from './lib/bridge'
import {startRegionTracking} from './overlay/regions'

const MARGIN = 16
const GAP = 14
const PANEL = {w: 352, h: 480} // w-[22rem] h-[30rem]
const BUBBLE_W = 288 // w-72
const BUBBLE_LINGER_MS = 3500

const clamp = (p: Point): Point => ({
  x: Math.min(Math.max(p.x, MARGIN), window.innerWidth - ORB_SIZE - MARGIN),
  y: Math.min(Math.max(p.y, MARGIN), window.innerHeight - ORB_SIZE - MARGIN),
})

// Right edge, a bit above the middle: away from the Dock and hot corners.
const defaultPosition = (): Point =>
  clamp({x: window.innerWidth - ORB_SIZE - 32, y: Math.round(window.innerHeight * 0.36)})

/** Opens the panel on the side of the screen that has room. */
function placement(p: Point) {
  const side: 'left' | 'right' = p.x + ORB_SIZE / 2 > window.innerWidth / 2 ? 'left' : 'right'
  const top = Math.min(
    Math.max(p.y + ORB_SIZE / 2 > window.innerHeight / 2 ? p.y + ORB_SIZE - PANEL.h : p.y, MARGIN),
    window.innerHeight - PANEL.h - MARGIN,
  )
  const dx = (w: number) => (side === 'left' ? -GAP - w : ORB_SIZE + GAP)
  return {side, panel: {left: dx(PANEL.w), top: top - p.y}, bubbleLeft: dx(BUBBLE_W)}
}

let nextId = 1

export default function App() {
  const [pos, setPos] = useState<Point | null>(null)
  const savedPos = useRef(false)
  const anchor = useRef<HTMLDivElement>(null)

  const [visible, setVisible] = useState(true)
  const [chatOpen, setChatOpen] = useState(false)
  const [speaking, setSpeaking] = useState(false)
  const [messages, setMessages] = useState<Message[]>([])
  const [waiting, setWaiting] = useState(false)
  const [seeScreen, setSeeScreen] = useState(true)
  const [aiReady, setAiReady] = useState(false)
  const [bubble, setBubble] = useState<{text: string; key: number} | null>(null)

  const chatOpenRef = useRef(chatOpen)
  chatOpenRef.current = chatOpen

  useEffect(() => startRegionTracking(), [])

  useEffect(() => {
    void bridge.isDebug().then((d) => document.body.classList.toggle('debug', d))
  }, [])

  // Restore the orb position, then start the welcome sequence.
  useEffect(() => {
    void bridge.getSettings().then((s) => {
      savedPos.current = !!s.orb
      setPos(s.orb ? clamp(s.orb) : defaultPosition())
      bridge.boot()
    })
  }, [])

  // The overlay is resized to full screen after load: follow it.
  useEffect(() => {
    const onResize = () => setPos((p) => (p && savedPos.current ? clamp(p) : defaultPosition()))
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  useEffect(() => bridge.onToggleChat(() => setChatOpen((o) => !o)), [])
  useEffect(
    () =>
      bridge.onVisibility((v) => {
        setVisible(v)
        if (!v) setChatOpen(false)
      }),
    [],
  )
  useEffect(
    () =>
      bridge.onSay((text) => {
        setMessages((m) => [...m, {id: `say-${nextId++}`, from: 'aria', text, at: new Date()}])
        if (!chatOpenRef.current) setBubble({text, key: nextId++})
      }),
    [],
  )
  useEffect(() => bridge.onSpeaking(setSpeaking), [])

  useEffect(() => {
    void bridge.status().then((s) => setAiReady(s.aiReady))
  }, [])

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

  // Keyboard focus follows the chat: taken on open, handed back on close.
  // Compares with the last notified value: StrictMode runs effects twice.
  const notified = useRef(chatOpen)
  useEffect(() => {
    if (chatOpen) setBubble(null)
    if (notified.current === chatOpen) return
    notified.current = chatOpen
    bridge.setChatOpen(chatOpen)
  }, [chatOpen])

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
    getOrigin: () => pos ?? defaultPosition(),
    clamp,
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

  if (!pos) return null
  const place = placement(pos)

  // Everything outside [data-interactive] elements is click-through.
  return (
    <main className="pointer-events-none fixed inset-0" hidden={!visible}>
      <div ref={anchor} className="pointer-events-auto absolute top-0 left-0 will-change-transform">
        <Orb open={chatOpen} speaking={speaking} handlers={drag} />

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
            onClose={() => setChatOpen(false)}
          />
        </div>

        {bubble && !chatOpen && (
          <div className="absolute top-1/2 -translate-y-1/2" style={{left: place.bubbleLeft}}>
            <Bubble key={bubble.key} text={bubble.text} speaking={speaking} side={place.side} />
          </div>
        )}
      </div>
    </main>
  )
}
