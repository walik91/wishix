import { requestJSON } from './client.js'

const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
export const searchPeople = (query, cursor = '') => requestJSON(`/search/people?${new URLSearchParams({ q: query, after: cursor })}`)
export const getProfileName = () => requestJSON('/profile/name')
export const setProfileName = display_name => requestJSON('/profile/name', { method: 'PUT', headers, body: JSON.stringify({ display_name }) })
