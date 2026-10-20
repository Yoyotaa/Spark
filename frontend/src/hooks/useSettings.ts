import {useCallback, useEffect, useRef, useState} from 'react'
import * as bridge from '../lib/bridge'
import {applyAppearance, type Settings} from '../lib/settings'
import type {SaveState} from '../components/settings/SettingsPanel'

const SAVE_DELAY_MS = 250

/**
 * Settings are edited optimistically (instant UI) and saved automatically,
 * debounced; Go answers with the normalized values, which win.
 */
export function useSettings() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [saveState, setSaveState] = useState<SaveState>('idle')
  const [saveError, setSaveError] = useState<string | null>(null)
  const [hotkeyError, setHotkeyError] = useState<string | undefined>()
  const pending = useRef<Settings | null>(null)
  const timer = useRef(0)

  useEffect(() => {
    void bridge.getSettings().then(setSettings)
  }, [])

  useEffect(() => {
    if (settings) applyAppearance(settings.appearance)
  }, [settings?.appearance])

  const flush = useCallback(async () => {
    const next = pending.current
    pending.current = null
    if (!next) return
    try {
      const res = await bridge.saveSettings(next)
      // Keep local edits made while saving; otherwise adopt the normalized values.
      if (!pending.current) setSettings(res.settings)
      setHotkeyError(res.hotkeyError)
      setSaveError(null)
      setSaveState('saved')
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e))
      setSaveState('error')
    }
  }, [])

  const update = useCallback(
    (fn: (draft: Settings) => void) => {
      setSettings((cur) => {
        if (!cur) return cur
        const draft = structuredClone(cur)
        fn(draft)
        pending.current = draft
        return draft
      })
      setSaveState('saving')
      clearTimeout(timer.current)
      timer.current = window.setTimeout(flush, SAVE_DELAY_MS)
    },
    [flush],
  )

  // Fade the "saved" indicator.
  useEffect(() => {
    if (saveState !== 'saved') return
    const id = setTimeout(() => setSaveState('idle'), 1800)
    return () => clearTimeout(id)
  }, [saveState])

  const reset = useCallback(async () => {
    const res = await bridge.resetSettings()
    setSettings(res.settings)
    setHotkeyError(res.hotkeyError)
    setSaveState('saved')
  }, [])

  return {settings, update, saveState, saveError, hotkeyError, reset}
}
