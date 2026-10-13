import authoring from './authoring'
import auth from './auth'
import admin from './admin'
import common from './common'
import community from './community'
import certificates from './certificates'
import courses from './courses'
import dashboard from './dashboard'
import errors from './errors'
import navigation from './navigation'
import session from './session'
import settings from './settings'
import learner from './learner'
import validation from './validation'

export const englishMessages = {
  admin,
  common,
  community,
  certificates,
  auth,
  navigation,
  session,
  settings,
  learner,
  authoring,
  courses,
  dashboard,
  validation,
  errors,
} as const

export default englishMessages
