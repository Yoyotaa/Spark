import {Component, type ReactNode} from 'react'
import {logFrontend} from '../lib/bridge'

type Props = {name: string; children: ReactNode}

/**
 * Contains a rendering error to one part of the overlay (chat, settings,
 * bubble) and reports it, so a bug never makes the whole interface vanish.
 */
export class Guard extends Component<Props, {failed: boolean}> {
  state = {failed: false}

  static getDerivedStateFromError() {
    return {failed: true}
  }

  componentDidCatch(error: Error, info: {componentStack?: string | null}) {
    logFrontend(`[${this.props.name}] ${error.message}\n${info.componentStack ?? ''}`)
  }

  render() {
    return this.state.failed ? null : this.props.children
  }
}
