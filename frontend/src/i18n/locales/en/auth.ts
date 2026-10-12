export default {
  checking: {
    title: 'Checking your session',
    description: 'One moment while we check whether you are already signed in.',
  },
  authenticated: {
    title: 'You are already signed in.',
    description: 'Continue to the Academy home page.',
    action: 'Continue to home',
  },
  unavailable: {
    title: 'Sign in unavailable',
    description: 'We couldn’t check your session right now. Please try again.',
    retry: 'Try again',
  },
  login: {
    title: 'Sign in',
    description: 'Use your Academy email address and password to continue.',
    email: 'Email',
    password: 'Password',
    submit: 'Sign in',
    submitting: 'Signing in…',
  },
  validation: {
    emailRequired: 'Enter your email address.',
    passwordRequired: 'Enter your password.',
  },
  errors: {
    invalidCredentials: 'The email or password is incorrect.',
    rateLimited: 'Too many sign-in attempts. Please try again {time}.',
    rateLimitedSoon: 'Too many sign-in attempts. Please try again shortly.',
    unavailable: 'We couldn’t sign you in right now. Please try again.',
  },
  sessionExpired: {
    title: 'Your session expired',
    description: 'Sign in again to continue securely.',
  },
} as const
