import {useEffect, useRef, useState} from 'react'
import {AriaOrb} from './components/AriaOrb'
import {ChatPanel} from './components/ChatPanel'
import {isDebug, onToggleChat, onVisibility, setChatOpen as notifyChatOpen} from './lib/bridge'
import {startRegionTracking} from './overlay/regions'

export default function App() {
  const [chatOpen, setChatOpen] = useState(false)
  const [visible, setVisible] = useState(true)

  useEffect(() => startRegionTracking(), [])

  useEffect(() => {
    void isDebug().then((d) => document.body.classList.toggle('debug', d))
  }, [])

  useEffect(() => onToggleChat(() => setChatOpen((o) => !o)), [])

  useEffect(
    () =>
      onVisibility((v) => {
        setVisible(v)
        if (!v) setChatOpen(false)
      }),
    [],
  )

  // Keyboard focus follows the chat: taken on open, handed back on close.
  // Compares with the last notified value rather than skipping the first run:
  // StrictMode runs effects twice in dev.
  const notified = useRef(chatOpen)
  useEffect(() => {
    if (notified.current === chatOpen) return
    notified.current = chatOpen
    notifyChatOpen(chatOpen)
  }, [chatOpen])

  // Everything outside [data-interactive] elements is click-through.
  return (
    <main className="pointer-events-none fixed inset-0" hidden={!visible}>
      <div className="pointer-events-auto absolute right-6 bottom-6 flex flex-col items-end gap-3">
        <ChatPanel open={chatOpen} onClose={() => setChatOpen(false)} />
        <AriaOrb active={chatOpen} onToggle={() => setChatOpen((o) => !o)} />
      </div>
    </main>
  )
}
