import { cleanup, render, screen } from '@testing-library/vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CourseWidgetBlock from './CourseWidgetBlock.vue'
import type { WidgetRuntimeLaunch } from '../plugins/runtime'

const request = vi.hoisted(() => vi.fn())
const launchDraftPreviewWidgetRuntime = vi.hoisted(() => vi.fn())
vi.mock('../api/client', async (original) => ({
  ...(await original<typeof import('../api/client')>()),
  apiClient: { request },
}))
vi.mock('../authoring/authoring', async (original) => ({
  ...(await original<typeof import('../authoring/authoring')>()),
  launchDraftPreviewWidgetRuntime,
}))
vi.mock('./WidgetRuntimeFrame.vue', () => ({
  default: {
    props: ['launch'],
    template: '<div data-testid="runtime-frame">{{ launch.draftPreviewContext?.configuration?.message ?? launch.courseContext?.configuration?.message }}</div>',
  },
}))

const draftID = '11111111-1111-4111-8111-111111111111'
const lessonID = '22222222-2222-4222-8222-222222222222'
const block = {
  key: 'preview-widget',
  type: 'PLUGIN_WIDGET' as const,
  payload: {
    pluginId: 'com.example.preview', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64),
    widgetId: 'timeline', widgetType: 'COURSE_WIDGET' as const, configuration: { ignored: true },
  },
}

function launch(overrides: Partial<WidgetRuntimeLaunch> = {}): WidgetRuntimeLaunch {
  return {
    context: { runtimeInstanceId: '33333333-3333-4333-8333-333333333333', pluginId: 'com.example.preview', pluginVersion: '1.0.0', artifactDigest: 'a'.repeat(64), widgetId: 'timeline', widgetType: 'COURSE_WIDGET' },
    widgetName: 'Timeline', runtimeUrl: 'https://plugins.academy.example/runtime', runtimeOrigin: 'https://plugins.academy.example', token: 'preview-token', expiresAt: '2026-09-28T12:05:00Z',
    capabilities: ['widget.runtime.bootstrap', 'widget.runtime.context.read', 'widget.course.preview.context.read'],
    draftPreviewContext: { contextType: 'DRAFT_PREVIEW', placementKey: block.key, configuration: { message: 'Preview-safe configuration' } },
    ...overrides,
  }
}

describe('CourseWidgetBlock Draft Preview runtime', () => {
  beforeEach(() => { request.mockReset(); launchDraftPreviewWidgetRuntime.mockReset() })
  afterEach(cleanup)

  it('launches from authoritative Draft placement identity without client coordinates or configuration', async () => {
    launchDraftPreviewWidgetRuntime.mockResolvedValue(launch())
    render(CourseWidgetBlock, { props: { block, draftId: draftID, draftLessonId: lessonID } })
    expect((await screen.findByTestId('runtime-frame')).textContent).toContain('Preview-safe configuration')
    expect(launchDraftPreviewWidgetRuntime).toHaveBeenCalledWith(draftID, lessonID, block.key)
    expect(request).not.toHaveBeenCalled()
  })

  it('fails closed when a Draft launch does not carry the dedicated Preview context', async () => {
    launchDraftPreviewWidgetRuntime.mockResolvedValue(launch({ draftPreviewContext: undefined, capabilities: ['widget.runtime.bootstrap', 'widget.runtime.context.read'] }))
    render(CourseWidgetBlock, { props: { block, draftId: draftID, draftLessonId: lessonID } })
    expect((await screen.findByText('Widget unavailable in preview.')).textContent).toContain('Widget unavailable in preview.')
    expect(screen.queryByTestId('runtime-frame')).toBeNull()
  })

  it('uses the unchanged published launch route outside Draft Preview', async () => {
    request.mockResolvedValue(launch({
      capabilities: ['widget.runtime.bootstrap', 'widget.runtime.context.read', 'widget.course.context.read'],
      draftPreviewContext: undefined,
      courseContext: { courseId: '55555555-5555-4555-8555-555555555555', courseVersionId: '44444444-4444-4444-8444-444444444444', courseVersion: '1.0.0', lessonKey: 'lesson-one', placementKey: block.key, presentationLanguage: 'en', configuration: { message: 'Published configuration' } },
    }))
    render(CourseWidgetBlock, { props: { block, courseId: '55555555-5555-4555-8555-555555555555', version: '1.0.0', lessonKey: 'lesson-one' } })
    expect((await screen.findByTestId('runtime-frame')).textContent).toContain('Published configuration')
    expect(request.mock.calls[0]![0]).toBe('/api/courses/by-id/55555555-5555-4555-8555-555555555555/versions/1.0.0/lessons/lesson-one/blocks/preview-widget/widget-runtime')
  })

  it('uses one calm unavailable state when Preview launch is denied', async () => {
    launchDraftPreviewWidgetRuntime.mockResolvedValue({ problem: { detail: 'approval fingerprint or digest detail' } })
    render(CourseWidgetBlock, { props: { block, draftId: draftID, draftLessonId: lessonID } })
    expect((await screen.findByText('Widget unavailable in preview.')).textContent).toContain('Widget unavailable in preview.')
    expect(document.body.textContent).not.toMatch(/fingerprint|digest/i)
  })
})
