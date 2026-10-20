// Thin wrapper over the generated Wails bindings so the UI also runs in a
// plain browser (`npm run dev`) where window.go does not exist.
import * as App from '../../wailsjs/go/app/App'
import type {overlay} from '../../wailsjs/go/models'

const inWails = typeof window !== 'undefined' && 'go' in window

export type Region = Pick<overlay.Rect, 'x' | 'y' | 'w' | 'h'>

export function setInteractiveRegions(regions: Region[]): void {
  if (inWails) void App.SetInteractiveRegions(regions as overlay.Rect[])
}

export function setMouseCaptured(v: boolean): void {
  if (inWails) void App.SetMouseCaptured(v)
}

export async function isDebug(): Promise<boolean> {
  return inWails ? App.Debug() : true
}
