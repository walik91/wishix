import { requestJSON } from './client.js'
const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
const post = (path, body) => requestJSON(`/reservations/${path}`, { method: 'POST', headers, body: JSON.stringify(body), signal: AbortSignal.timeout(20000) })
export const startReservation = body => post('start', body)
export const confirmReservation = body => post('confirm', body)
export const recoverReservation = body => post('recover', body)
export const manageReservation = (token, action = '') => post('manage', { token, action })
