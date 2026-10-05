import authoring from './authoring'
import common from './common'
import courses from './courses'
import dashboard from './dashboard'
import errors from './errors'
import navigation from './navigation'
import session from './session'
import validation from './validation'

export const englishMessages = {
  common,
  navigation,
  session,
  authoring,
  courses,
  dashboard,
  validation,
  errors,
} as const

export default englishMessages
