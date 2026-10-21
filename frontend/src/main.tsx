import React from 'react'
import {createRoot} from 'react-dom/client'
import '@fontsource-variable/space-grotesk'
import '@fontsource-variable/jetbrains-mono'
import './style.css'
import App from './App'
import {logFrontend} from './lib/bridge'

window.addEventListener('error', (e) => logFrontend(`${e.message} (${e.filename}:${e.lineno})`))
window.addEventListener('unhandledrejection', (e) => logFrontend(`promesse rejetée : ${String(e.reason)}`))

// Outside Wails (`npm run dev` in a browser) there is no desktop behind the
// transparent page: paint a fake one to judge the design.
if (!('go' in window)) document.body.classList.add('preview')

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
