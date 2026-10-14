import type { components } from '../api/generated'
import { useAuth, type AuthService } from '../auth/auth'

export type TranslationSummary = components['schemas']['TranslationSummary']
export type TranslationList = components['schemas']['TranslationList']
export type TranslationPublication = components['schemas']['TranslationPublication']
export type TranslationChange = components['schemas']['TranslationTextChange']
export type TranslationWorkspace = components['schemas']['TranslationWorkspace']
export type Completeness = components['schemas']['TranslationCompleteness']
export type TextField = components['schemas']['TranslationTextField']
export type TranslationCourse = components['schemas']['TranslationCourse']
export type TranslationModule = components['schemas']['TranslationModule']
export type TranslationLesson = components['schemas']['TranslationLesson']
export type TranslationBlock = components['schemas']['TranslationContentBlock']
export type TranslationAssessment = components['schemas']['TranslationAssessment']
export type TranslationQuestion = components['schemas']['TranslationQuestion']
export type TranslationItem = components['schemas']['TranslationItem']
export class InvalidTranslationResponseError extends Error { constructor(){super('Invalid Translation response')} }
const json = (method:string, body:unknown) => ({ method, csrf:true, headers:{'Content-Type':'application/json'}, body:JSON.stringify(body), cache:'no-store' as RequestCache })
const obj=(x:unknown): x is Record<string,unknown> => typeof x==='object' && x!==null && !Array.isArray(x)
const str=(x:unknown): x is string => typeof x==='string'
const strings=(x:unknown): x is string[] => Array.isArray(x)&&x.every(str)
const complete=(x:unknown): x is Completeness => obj(x)&&Number.isInteger(x.totalTranslatableFields)&&Number.isInteger(x.translatedFields)&&Number.isInteger(x.untranslatedFields)&&typeof x.complete==='boolean'
const field=(x:unknown): x is TextField => obj(x)&&str(x.source)&&(x.translated===null||str(x.translated))&&(x.state==='UNTRANSLATED'||x.state==='TRANSLATED')
const item=(x:unknown): x is TranslationItem => obj(x)&&str(x.sourceStableKey)&&Number.isInteger(x.position)&&field(x.text)
const question=(x:unknown): x is TranslationQuestion => obj(x)&&str(x.sourceStableKey)&&str(x.type)&&Number.isInteger(x.position)&&field(x.prompt)&&Array.isArray(x.options)&&x.options.every(item)&&Array.isArray(x.leftItems)&&x.leftItems.every(item)&&Array.isArray(x.rightItems)&&x.rightItems.every(item)
const block=(x:unknown): x is TranslationBlock => obj(x)&&str(x.sourceBlockKey)&&str(x.type)&&Number.isInteger(x.position)&&obj(x.fields)&&Object.values(x.fields).every(field)&&strings(x.assetKeys)&&str(x.code)&&strings(x.tableHeaders)&&Array.isArray(x.tableRows)&&x.tableRows.every(strings)&&str(x.assessmentKey)
const lesson=(x:unknown): x is TranslationLesson => obj(x)&&str(x.sourceStableKey)&&Number.isInteger(x.position)&&strings(x.prerequisiteStableKeys)&&field(x.title)&&field(x.description)&&Array.isArray(x.learningObjectives)&&x.learningObjectives.every(field)&&Array.isArray(x.blocks)&&x.blocks.every(block)&&complete(x.completeness)
const module=(x:unknown): x is TranslationModule => obj(x)&&str(x.sourceStableKey)&&Number.isInteger(x.position)&&field(x.title)&&field(x.description)&&Array.isArray(x.lessons)&&x.lessons.every(lesson)&&complete(x.completeness)
function workspace(x:unknown): x is TranslationWorkspace { return obj(x)&&str(x.translationId)&&Number.isInteger(x.revision)&&(x.lifecycle==='DRAFT'||x.lifecycle==='PUBLISHED')&&str(x.targetLanguage)&&obj(x.source)&&str(x.source.courseId)&&str(x.source.courseVersionId)&&str(x.source.version)&&str(x.source.language)&&obj(x.course)&&field(x.course.title)&&field(x.course.description)&&Array.isArray(x.course.learningObjectives)&&x.course.learningObjectives.every(field)&&Array.isArray(x.modules)&&x.modules.every(module)&&Array.isArray(x.assessments)&&x.assessments.every((a)=>obj(a)&&str(a.assessmentKey)&&Array.isArray(a.questions)&&a.questions.every(question)&&complete(a.completeness))&&complete(x.completeness) }
export async function listTranslations(courseId:string,version:string, client:Pick<AuthService,'request'>=useAuth()):Promise<TranslationList>{const x=await client.request<unknown>(`/api/courses/by-id/${encodeURIComponent(courseId)}/versions/${encodeURIComponent(version)}/translations`,{cache:'no-store'});if(!obj(x)||!Array.isArray(x.translations))throw new InvalidTranslationResponseError();return x as TranslationList}
export async function createTranslation(courseId:string,version:string,targetLanguage:string,client:Pick<AuthService,'request'>=useAuth()):Promise<TranslationSummary>{const x=await client.request<unknown>(`/api/courses/by-id/${encodeURIComponent(courseId)}/versions/${encodeURIComponent(version)}/translations`,json('POST',{targetLanguage}));if(!obj(x)||!str(x.translationId)||!Number.isInteger(x.revision))throw new InvalidTranslationResponseError();return x as TranslationSummary}
export async function getTranslation(id:string,client:Pick<AuthService,'request'>=useAuth()):Promise<TranslationWorkspace>{const x=await client.request<unknown>(`/api/translations/${encodeURIComponent(id)}`,{cache:'no-store'});if(!workspace(x))throw new InvalidTranslationResponseError();return x}
export async function patchTranslation(id:string,expectedRevision:number,changes:TranslationChange[],client:Pick<AuthService,'request'>=useAuth()):Promise<TranslationSummary>{const x=await client.request<unknown>(`/api/translations/${encodeURIComponent(id)}`,json('PATCH',{expectedRevision,changes}));if(!obj(x)||!str(x.translationId)||!Number.isInteger(x.revision))throw new InvalidTranslationResponseError();return x as TranslationSummary}
export async function publishTranslation(id:string,expectedRevision:number,client:Pick<AuthService,'request'>=useAuth()):Promise<TranslationPublication>{const x=await client.request<unknown>(`/api/translations/${encodeURIComponent(id)}/publish`,json('POST',{expectedRevision}));if(!obj(x)||!str(x.publicationId)||!str(x.translationId))throw new InvalidTranslationResponseError();return x as TranslationPublication}
