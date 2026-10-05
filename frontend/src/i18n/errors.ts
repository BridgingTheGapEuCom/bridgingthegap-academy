export type ApplicationErrorKey = `errors.${'generic' | 'unavailable' | 'forbidden' | 'notFound' | 'conflict'}`

const statusKeys: Partial<Record<number, ApplicationErrorKey>> = {
  403: 'errors.forbidden',
  404: 'errors.notFound',
  409: 'errors.conflict',
  503: 'errors.unavailable',
}

export function applicationErrorKey(error: unknown, codeKeys: Readonly<Record<string, ApplicationErrorKey>> = {}): ApplicationErrorKey {
  if (!error || typeof error !== 'object') return 'errors.generic'
  const candidate = error as { status?: unknown; problem?: { code?: unknown } }
  const code = candidate.problem?.code
  if (typeof code === 'string' && codeKeys[code]) return codeKeys[code]
  return typeof candidate.status === 'number' ? statusKeys[candidate.status] ?? 'errors.generic' : 'errors.generic'
}

export function validationMessageKey(code: string): `validation.${'required' | 'tooLong' | 'invalidLanguageTag'}` | undefined {
  if (code === 'required' || code === 'tooLong' || code === 'invalidLanguageTag') return `validation.${code}`
  return undefined
}
