import {useState, type FormEvent} from 'react'

type Message = {id: number; from: 'user' | 'aria'; text: string}

let nextId = 1

export function ChatPanel() {
  const [messages, setMessages] = useState<Message[]>([
    {id: 0, from: 'aria', text: 'A.R.I.A en ligne. Je vois ton écran dès que tu me le demandes.'},
  ])
  const [draft, setDraft] = useState('')

  function submit(e: FormEvent) {
    e.preventDefault()
    const text = draft.trim()
    if (!text) return
    setDraft('')
    setMessages((m) => [
      ...m,
      {id: nextId++, from: 'user', text},
      // Placeholder until the AI module (phase 3) is wired.
      {id: nextId++, from: 'aria', text: 'Module IA pas encore connecté.'},
    ])
  }

  return (
    <section
      data-interactive
      className="flex h-96 w-80 flex-col overflow-hidden rounded-2xl border border-aria/30 bg-panel text-sm text-slate-100 shadow-2xl backdrop-blur-xl"
    >
      <header className="border-b border-white/10 px-4 py-2.5 text-xs font-semibold tracking-[0.3em] text-aria">
        A.R.I.A
      </header>
      <ol className="flex-1 space-y-2 overflow-y-auto px-4 py-3 select-text">
        {messages.map((m) => (
          <li
            key={m.id}
            className={`max-w-[85%] rounded-xl px-3 py-1.5 ${
              m.from === 'user' ? 'ml-auto bg-aria/25' : 'bg-white/5 text-slate-200'
            }`}
          >
            {m.text}
          </li>
        ))}
      </ol>
      <form onSubmit={submit} className="border-t border-white/10 p-2">
        <input
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Demande à A.R.I.A…"
          className="w-full rounded-lg bg-white/5 px-3 py-2 outline-none placeholder:text-slate-500 focus:ring-1 focus:ring-aria/60"
        />
      </form>
    </section>
  )
}
