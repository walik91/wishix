import { requestJSON } from './client.js'

const post = (action, body) => requestJSON(`/auth/${action}`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' },
  body: JSON.stringify(body),
  signal: AbortSignal.timeout(20000),
})

export const register = data => post('register', data)
export const verifyEmail = (challenge, code) => post('verify-email', { challenge, code })
export const resendCode = challenge => post('resend', { challenge })
export const login = data => post('login', data)
export const logout = () => post('logout', {})
export const currentUser = () => requestJSON('/auth/me')
export const forgotPassword = data => post('forgot-password', data)
export const resetPassword = data => post('reset-password', data)
export const googleStatus = () => requestJSON('/auth/google/status')
export const startGoogle = mode => post('google/start', { mode })
export const completeGoogle = username => post('google/complete', { username })
