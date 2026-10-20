import {useEffect, useState} from 'react'
import {AriaOrb} from './components/AriaOrb'
import {ChatPanel} from './components/ChatPanel'
import {isDebug} from './lib/bridge'
import {startRegionTracking} from './overlay/regions'

export default function App() {
  const [chatOpen, setChatOpen] = useState(false)

  useEffect(() => startRegionTracking(), [])

  useEffect(() => {
    void isDebug().then((d) => document.body.classList.toggle('debug', d))
  }, [])

  // Everything outside [data-interactive] elements is click-through.
  return (
    <main className="pointer-events-none fixed inset-0">
      <div className="pointer-events-auto absolute right-6 bottom-6 flex flex-col items-end gap-3">
        {chatOpen && <ChatPanel />}
        <AriaOrb active={chatOpen} onToggle={() => setChatOpen((o) => !o)} />
      </div>
    </main>
  )
}
