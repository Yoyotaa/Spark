// Tracks every element marked with `data-interactive` and reports their
// bounding boxes to Go, which uses them to decide when the overlay should
// capture the mouse.
//
// Performance: all triggers are coalesced into a single requestAnimationFrame,
// and Go is only called when the rectangles actually changed.
import {setInteractiveRegions, type Region} from '../lib/bridge'

const SELECTOR = '[data-interactive]'
const PADDING = 4 // px of slack so fast cursor moves don't miss the edge

let frame = 0
let last = ''
let resizeObserver: ResizeObserver | null = null
const observed = new Set<Element>()

function collect(): Region[] {
  const regions: Region[] = []
  document.querySelectorAll(SELECTOR).forEach((el) => {
    const r = el.getBoundingClientRect()
    if (r.width === 0 || r.height === 0) return
    regions.push({
      x: Math.floor(r.left - PADDING),
      y: Math.floor(r.top - PADDING),
      w: Math.ceil(r.width + PADDING * 2),
      h: Math.ceil(r.height + PADDING * 2),
    })
  })
  return regions
}

function syncObserved() {
  const current = new Set(document.querySelectorAll(SELECTOR))
  for (const el of observed) {
    if (!current.has(el)) {
      resizeObserver?.unobserve(el)
      observed.delete(el)
    }
  }
  for (const el of current) {
    if (!observed.has(el)) {
      resizeObserver?.observe(el)
      observed.add(el)
    }
  }
}

function flush() {
  frame = 0
  syncObserved()
  const regions = collect()
  const key = JSON.stringify(regions)
  if (key === last) return
  last = key
  setInteractiveRegions(regions)
}

/** Schedules a region update on the next frame (idempotent). */
export function scheduleRegionsUpdate() {
  if (frame === 0) frame = requestAnimationFrame(flush)
}

/** Starts tracking; returns a disposer. */
export function startRegionTracking(): () => void {
  resizeObserver = new ResizeObserver(scheduleRegionsUpdate)
  const mutations = new MutationObserver(scheduleRegionsUpdate)
  mutations.observe(document.body, {
    subtree: true,
    childList: true,
    attributes: true,
    attributeFilter: ['data-interactive', 'class', 'style'],
  })
  // CSS transitions/animations move elements without mutating the DOM.
  const events = ['resize', 'transitionend', 'animationend'] as const
  events.forEach((e) => window.addEventListener(e, scheduleRegionsUpdate, true))
  scheduleRegionsUpdate()

  return () => {
    mutations.disconnect()
    resizeObserver?.disconnect()
    resizeObserver = null
    observed.clear()
    events.forEach((e) => window.removeEventListener(e, scheduleRegionsUpdate, true))
    if (frame) cancelAnimationFrame(frame)
    frame = 0
    last = ''
  }
}
