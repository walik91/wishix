import { requestJSON } from './client.js'

const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
export const listWishes = () => requestJSON('/wishes')
export const createWish = data => requestJSON('/wishes', {
  method: 'POST', headers, body: JSON.stringify(data), signal: AbortSignal.timeout(20000),
})
export const deleteWish = id => requestJSON(`/wishes/${encodeURIComponent(id)}`, {
  method: 'DELETE', headers, body: '{}', signal: AbortSignal.timeout(20000),
})
