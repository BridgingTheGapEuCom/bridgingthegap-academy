export const widgetRuntimeProtocol = 'btg-widget-runtime' as const
export const widgetRuntimeProtocolVersion = 1 as const

export type WidgetRuntimeContext = {
  runtimeInstanceId: string
  pluginId: string
  pluginVersion: string
  artifactDigest: string
  widgetId: string
  widgetType: 'COURSE_WIDGET' | 'DASHBOARD_WIDGET'
}

export type WidgetRuntimeLaunch = {
  context: WidgetRuntimeContext
  widgetName: string
  runtimeUrl: string
  runtimeOrigin: string
  token: string
  expiresAt: string
  capabilities: string[]
  courseContext?: {
    courseId: string
    courseVersionId: string
    courseVersion: string
    lessonKey: string
    placementKey: string
    presentationLanguage: string
    configuration: Record<string, unknown>
  }
  dashboardContext?: {
    placementId: string
    configuration: Record<string, unknown>
  }
}

export type WidgetRuntimeMessage = {
  protocol: typeof widgetRuntimeProtocol
  version: typeof widgetRuntimeProtocolVersion
  type: 'WIDGET_READY' | 'RUNTIME_INITIALIZED' | 'RUNTIME_ERROR' | 'BTG_RUNTIME_DISABLED'
  runtimeInstanceId: string
  payload: unknown
}

export function isSafeRuntimeLaunch(value: WidgetRuntimeLaunch): boolean {
  try {
    const runtimeOrigin = new URL(value.runtimeOrigin)
    const runtimeUrl = new URL(value.runtimeUrl)
    return (
      runtimeOrigin.origin === value.runtimeOrigin &&
      runtimeUrl.origin === runtimeOrigin.origin &&
      runtimeOrigin.origin !== window.location.origin &&
      value.context.runtimeInstanceId.length > 0 &&
      value.widgetName.trim().length > 0 &&
      value.token.length > 0 &&
      ['COURSE_WIDGET', 'DASHBOARD_WIDGET'].includes(value.context.widgetType)
    )
  } catch {
    return false
  }
}

export function runtimeMessageFrom(
  event: MessageEvent,
  frame: HTMLIFrameElement,
  launch: WidgetRuntimeLaunch,
): WidgetRuntimeMessage | undefined {
  if (event.source !== frame.contentWindow || event.origin !== launch.runtimeOrigin) return
  const value: unknown = event.data
  if (!value || typeof value !== 'object') return
  const message = value as Record<string, unknown>
  if (
    message.protocol !== widgetRuntimeProtocol ||
    message.version !== widgetRuntimeProtocolVersion ||
    message.runtimeInstanceId !== launch.context.runtimeInstanceId ||
    typeof message.type !== 'string' ||
    !['WIDGET_READY', 'RUNTIME_INITIALIZED', 'RUNTIME_ERROR', 'BTG_RUNTIME_DISABLED'].includes(
      message.type,
    ) ||
    !('payload' in message)
  ) return
  return message as WidgetRuntimeMessage
}
