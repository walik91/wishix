import { requestJSON } from './client.js'
const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
const mutate = (path, method, value) => requestJSON(path, { method, headers, body: JSON.stringify(value), signal: AbortSignal.timeout(20000) })
const person = username => `/social/${encodeURIComponent(username)}`
export const getProfileDetails = () => requestJSON('/profile/details')
export const saveProfileDetails = value => mutate('/profile/details', 'PUT', value)
export const changeUsername = username => mutate('/profile/username', 'PUT', { username })
export const publicDetails = username => requestJSON(`/public/profiles/${encodeURIComponent(username)}/details`)
export const socialState = username => requestJSON(`${person(username)}/state`)
export const setFollow = (username, follow) => mutate(`${person(username)}/follow`, 'PUT', { follow })
export const setBlock = (username, block) => mutate(`${person(username)}/block`, 'PUT', { block })
export const sendReport = (username, message) => mutate(`${person(username)}/report`, 'POST', { message })
export const socialPeople = (username, kind, cursor = '') => requestJSON(`${person(username)}/people/${encodeURIComponent(kind)}?${new URLSearchParams({ after: cursor })}`)
export const uploadAvatar = file => requestJSON('/profile/avatar', { method: 'PUT', headers: { 'Content-Type': file.type, 'X-Wishix-Request': '1' }, body: file, signal: AbortSignal.timeout(25000) })
export const deleteAvatar = () => mutate('/profile/avatar', 'DELETE', {})
export const publicAvatarURL = username => `/api/public/profiles/${encodeURIComponent(username)}/avatar`
