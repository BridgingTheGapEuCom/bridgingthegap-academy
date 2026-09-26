import { APIProblemError, apiClient, type APIClient } from '../api/client'
import type { components } from '../api/generated'
import { InvalidCourseRouteError, InvalidPublishedCourseResponseError, isPublishedCourseID } from './courses'

export type CourseCommunity = components['schemas']['CourseCommunity']
export type CommunityThreadPage = components['schemas']['CommunityThreadPage']
export type CommunityThread = components['schemas']['CommunityThread']
export type CommunityPost = components['schemas']['CommunityPost']
export type CommunityModerationProbe = components['schemas']['CommunityModerationProbe']
export type CommunityModeratorThreadPage = components['schemas']['CommunityModeratorThreadPage']
export type CommunityModeratorThread = components['schemas']['CommunityModeratorThread']

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
const pageSize = 20

export class CommunityDisabledError extends Error { constructor() { super('Community is disabled') } }
export const communityPageSize = pageSize
export function communityPath(courseId: string) { return `/courses/by-id/${encodeURIComponent(courseId)}/community` }
export function communityThreadPath(courseId: string, threadId: string) { return `${communityPath(courseId)}/threads/${encodeURIComponent(threadId)}` }

export async function getCourseCommunity(courseId: string, client: APIClient = apiClient): Promise<CourseCommunity> {
  return request(client, courseId, '', isCommunity)
}
export async function listCommunityThreads(courseId: string, offset = 0, client: APIClient = apiClient): Promise<CommunityThreadPage> {
  if (!Number.isSafeInteger(offset) || offset < 0) throw new InvalidCourseRouteError()
  return request(client, courseId, `/threads?limit=${pageSize}&offset=${offset}`, isThreadPage)
}
export async function getCommunityThread(courseId: string, threadId: string, offset = 0, client: APIClient = apiClient): Promise<CommunityThread> {
  if (!isUUID(threadId) || !Number.isSafeInteger(offset) || offset < 0) throw new InvalidCourseRouteError()
  return request(client, courseId, `/threads/${encodeURIComponent(threadId)}?limit=${pageSize}&offset=${offset}`, isThread)
}
export async function createCommunityThread(courseId: string, title: string, body: string, client: APIClient = apiClient): Promise<CommunityThread> {
  return request(client, courseId, '/threads', isThread, { method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ title, body }) })
}
export async function createCommunityPost(courseId: string, threadId: string, body: string, client: APIClient = apiClient): Promise<CommunityPost> {
  if (!isUUID(threadId)) throw new InvalidCourseRouteError()
  return request(client, courseId, `/threads/${encodeURIComponent(threadId)}/posts`, isPost, { method: 'POST', cache: 'no-store', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ body }) })
}
export function communityModerationPath(courseId: string) { return `${communityPath(courseId)}/moderation` }
export function communityModeratorThreadPath(courseId: string, threadId: string) { return `${communityModerationPath(courseId)}/threads/${encodeURIComponent(threadId)}` }
export async function getCommunityModerationProbe(courseId:string,client:APIClient=apiClient):Promise<CommunityModerationProbe>{return request(client,courseId,'/moderation',isProbe)}
export async function listCommunityThreadsForModeration(courseId:string,offset=0,client:APIClient=apiClient):Promise<CommunityModeratorThreadPage>{return request(client,courseId,`/moderation/threads?limit=${pageSize}&offset=${offset}`,isModeratorPage)}
export async function getCommunityThreadForModeration(courseId:string,threadId:string,client:APIClient=apiClient):Promise<CommunityModeratorThread>{if(!isUUID(threadId))throw new InvalidCourseRouteError();return request(client,courseId,`/moderation/threads/${encodeURIComponent(threadId)}?limit=${pageSize}&offset=0`,isModeratorThread)}
export async function hideCommunityThread(course:string,thread:string,client:APIClient=apiClient){return moderationMutation(course,thread,'', 'hide',client)}
export async function unhideCommunityThread(course:string,thread:string,client:APIClient=apiClient){return moderationMutation(course,thread,'', 'unhide',client)}
export async function hideCommunityPost(course:string,thread:string,post:string,client:APIClient=apiClient){return moderationMutation(course,thread,post,'hide',client)}
export async function unhideCommunityPost(course:string,thread:string,post:string,client:APIClient=apiClient){return moderationMutation(course,thread,post,'unhide',client)}
async function moderationMutation(course:string,thread:string,post:string,action:string,client:APIClient){if(!isPublishedCourseID(course)||!isUUID(thread)||(post&&!isUUID(post)))throw new InvalidCourseRouteError();return client.request<unknown>(`/api/courses/by-id/${encodeURIComponent(course)}/community/threads/${encodeURIComponent(thread)}${post?`/posts/${encodeURIComponent(post)}`:''}/${action}`,{method:'POST',cache:'no-store'})}
async function request<T>(client: APIClient, courseId: string, suffix: string, validator: (value: unknown) => value is T, options: Parameters<APIClient['request']>[1] = { cache: 'no-store' }): Promise<T> {
  if (!isPublishedCourseID(courseId)) throw new InvalidCourseRouteError()
  try {
    const value = await client.request<unknown>(`/api/courses/by-id/${encodeURIComponent(courseId)}/community${suffix}`, options)
    if (!validator(value)) throw new InvalidPublishedCourseResponseError()
    return value
  } catch (error) {
    if (error instanceof APIProblemError && error.status === 409 && error.problem?.code === 'community_disabled') throw new CommunityDisabledError()
    throw error
  }
}
function isCommunity(value: unknown): value is CourseCommunity { return record(value) && only(value, ['courseId', 'mode']) && isUUID(value.courseId) && (value.mode === 'ENABLED' || value.mode === 'DISABLED') }
function isProbe(v:unknown):v is CommunityModerationProbe{return record(v)&&only(v,['canModerate'])&&typeof v.canModerate==='boolean'}
function isModeratorPage(v:unknown):v is CommunityModeratorThreadPage{return record(v)&&only(v,['threads','total','limit','offset'])&&Array.isArray(v.threads)&&v.threads.every((x)=>record(x)&&isUUID(x.threadId)&&text(x.title,240)&&isAuthor(x.author)&&(x.state==='VISIBLE'||x.state==='HIDDEN'))&&integer(v.total,0)&&integer(v.limit,1)&&integer(v.offset,0)}
function isModeratorThread(v:unknown):v is CommunityModeratorThread{return record(v)&&only(v,['threadId','title','author','state','createdAt','updatedAt','posts','postTotal'])&&isUUID(v.threadId)&&text(v.title,240)&&isAuthor(v.author)&&(v.state==='VISIBLE'||v.state==='HIDDEN')&&Array.isArray(v.posts)&&v.posts.every((p)=>record(p)&&isUUID(p.postId)&&isAuthor(p.author)&&text(p.body,20000)&&(p.state==='VISIBLE'||p.state==='HIDDEN')&&typeof p.isOpeningPost==='boolean'&&date(p.createdAt)&&date(p.updatedAt))&&integer(v.postTotal,0)}
function isThreadPage(value: unknown): value is CommunityThreadPage { return record(value) && only(value, ['threads', 'total', 'limit', 'offset']) && Array.isArray(value.threads) && value.threads.every(isThreadSummary) && integer(value.total, 0) && integer(value.limit, 1) && integer(value.offset, 0) }
function isThreadSummary(value: unknown): boolean { return record(value) && only(value, ['threadId','title','author','postCount','createdAt','updatedAt']) && isUUID(value.threadId) && text(value.title,240) && isAuthor(value.author) && integer(value.postCount,0) && date(value.createdAt) && date(value.updatedAt) }
function isThread(value: unknown): value is CommunityThread { return record(value) && only(value,['threadId','title','author','createdAt','updatedAt','posts','postTotal']) && isUUID(value.threadId) && text(value.title,240) && isAuthor(value.author) && date(value.createdAt) && date(value.updatedAt) && Array.isArray(value.posts) && value.posts.every(isPost) && integer(value.postTotal,0) }
function isPost(value: unknown): value is CommunityPost { return record(value) && only(value,['postId','author','body','createdAt','updatedAt']) && isUUID(value.postId) && isAuthor(value.author) && text(value.body,20000) && date(value.createdAt) && date(value.updatedAt) }
function isAuthor(value: unknown): boolean { return record(value) && only(value,['userId']) && isUUID(value.userId) }
function isUUID(value: unknown): value is string { return typeof value === 'string' && uuid.test(value) }
function text(value: unknown, max: number): boolean { return typeof value === 'string' && value.trim().length > 0 && value.length <= max }
function date(value: unknown): boolean { return typeof value === 'string' && !Number.isNaN(Date.parse(value)) }
function integer(value: unknown, min: number): boolean { return typeof value === 'number' && Number.isSafeInteger(value) && value >= min }
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function only(value: Record<string, unknown>, keys: string[]): boolean { return Object.keys(value).every((key) => keys.includes(key)) }
