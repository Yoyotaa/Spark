import {useRef, type PointerEvent} from 'react'
import {setMouseCaptured, type Point} from '../lib/bridge'

const THRESHOLD = 4 // px before a press becomes a drag

type Options = {
  getOrigin: () => Point
  /** Called on every move with the clamped position; keep it cheap. */
  onMove: (p: Point) => void
  onDrop: (p: Point) => void
  onTap: () => void
  clamp: (p: Point) => Point
}

type DragState = {pointer: Point; origin: Point; last: Point; moved: boolean}

/**
 * Press-and-drag that also supports plain clicks. While dragging, the
 * overlay keeps the whole window's mouse capture: otherwise a fast move
 * leaves the button's region and the OS would swallow the rest of the drag.
 */
export function useDrag({getOrigin, onMove, onDrop, onTap, clamp}: Options) {
  const state = useRef<DragState | null>(null)

  return {
    onPointerDown(e: PointerEvent<HTMLElement>) {
      if (e.button !== 0) return
      e.currentTarget.setPointerCapture(e.pointerId)
      setMouseCaptured(true)
      const origin = getOrigin()
      state.current = {pointer: {x: e.clientX, y: e.clientY}, origin, last: origin, moved: false}
    },
    onPointerMove(e: PointerEvent<HTMLElement>) {
      const s = state.current
      if (!s) return
      const dx = e.clientX - s.pointer.x
      const dy = e.clientY - s.pointer.y
      if (!s.moved && Math.hypot(dx, dy) < THRESHOLD) return
      if (!s.moved) {
        s.moved = true
        e.currentTarget.dataset.dragging = ''
      }
      s.last = clamp({x: s.origin.x + dx, y: s.origin.y + dy})
      onMove(s.last)
    },
    onPointerUp(e: PointerEvent<HTMLElement>) {
      const s = state.current
      if (!s) return
      state.current = null
      delete e.currentTarget.dataset.dragging
      setMouseCaptured(false)
      if (s.moved) onDrop(s.last)
      else onTap()
    },
    onPointerCancel(e: PointerEvent<HTMLElement>) {
      const s = state.current
      state.current = null
      delete e.currentTarget.dataset.dragging
      setMouseCaptured(false)
      if (s?.moved) onDrop(s.last)
    },
  }
}
