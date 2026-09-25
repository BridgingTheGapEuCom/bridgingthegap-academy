import { cleanup, fireEvent, render } from '@testing-library/vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import WidgetRuntimeFrame from './WidgetRuntimeFrame.vue'
import type { WidgetRuntimeLaunch } from '../plugins/runtime'

const launch: WidgetRuntimeLaunch = {
  context: {
    runtimeInstanceId: '11111111-1111-4111-8111-111111111111',
    pluginId: 'com.example.academy.timeline',
    pluginVersion: '1.0.0',
    artifactDigest: 'a'.repeat(64),
    widgetId: 'timeline',
    widgetType: 'COURSE_WIDGET',
  },
  runtimeUrl: 'https://plugins.academy.example/plugins/runtime/example#runtime=111',
  runtimeOrigin: 'https://plugins.academy.example',
  token: 'short-lived-token',
  expiresAt: '2026-09-25T12:05:00Z',
  capabilities: ['widget.runtime.bootstrap'],
}

describe('WidgetRuntimeFrame', () => {
  afterEach(cleanup)

  it('uses the cross-origin script sandbox without navigation, popup, or form authority', () => {
    const { getByTitle } = render(WidgetRuntimeFrame, { props: { launch, title: 'Timeline' } })
    const frame = getByTitle('Timeline') as HTMLIFrameElement
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts allow-same-origin')
    expect(frame.getAttribute('sandbox')).not.toContain('allow-top-navigation')
    expect(frame.getAttribute('sandbox')).not.toContain('allow-popups')
    expect(frame.getAttribute('sandbox')).not.toContain('allow-forms')
    expect(frame.getAttribute('referrerpolicy')).toBe('no-referrer')
  })

  it('initializes only the expected frame and exact runtime origin', async () => {
    const { getByTitle } = render(WidgetRuntimeFrame, { props: { launch, title: 'Timeline' } })
    const frame = getByTitle('Timeline') as HTMLIFrameElement
    const postMessage = vi.spyOn(frame.contentWindow!, 'postMessage')
    const ready = { protocol: 'btg-widget-runtime', version: 1, type: 'WIDGET_READY', runtimeInstanceId: launch.context.runtimeInstanceId, payload: {} }

    window.dispatchEvent(new MessageEvent('message', { data: ready, origin: 'https://evil.example', source: frame.contentWindow }))
    window.dispatchEvent(new MessageEvent('message', { data: ready, origin: launch.runtimeOrigin, source: window }))
    window.dispatchEvent(new MessageEvent('message', { data: { ...ready, version: 2 }, origin: launch.runtimeOrigin, source: frame.contentWindow }))
    window.dispatchEvent(new MessageEvent('message', { data: { ...ready, type: 'UNKNOWN' }, origin: launch.runtimeOrigin, source: frame.contentWindow }))
    expect(postMessage).not.toHaveBeenCalled()

    window.dispatchEvent(new MessageEvent('message', { data: ready, origin: launch.runtimeOrigin, source: frame.contentWindow }))
    expect(postMessage).toHaveBeenCalledOnce()
    expect(postMessage).toHaveBeenCalledWith(expect.objectContaining({ type: 'RUNTIME_INIT' }), launch.runtimeOrigin)
    await fireEvent(frame, new Event('load'))
  })

  it('does not create a privileged same-origin frame from a malformed descriptor', () => {
    const unsafe = { ...launch, runtimeOrigin: window.location.origin, runtimeUrl: `${window.location.origin}/plugin.js` }
    const { queryByTitle } = render(WidgetRuntimeFrame, { props: { launch: unsafe, title: 'Unsafe' } })
    expect(queryByTitle('Unsafe')).toBeNull()
  })
})
