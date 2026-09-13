import type { paths } from './generated'

export type LiveResponse = paths['/health/live']['get']['responses'][200]['content']['application/json']

export async function getLiveness(): Promise<LiveResponse> {
  const response = await fetch('/health/live')
  if (!response.ok) throw new Error('The application is unavailable')
  return response.json() as Promise<LiveResponse>
}
