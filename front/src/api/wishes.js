import { requestJSON } from './client.js'

const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
export const listWishes = () => requestJSON('/wishes')
export function createWish(data, image = null) {
  if (!image) return requestJSON('/wishes', { method: 'POST', headers, body: JSON.stringify(data), signal: AbortSignal.timeout(20000) })
  const body = new FormData()
  body.append('wish', JSON.stringify(data))
  body.append('image', image)
  return requestJSON('/wishes', { method: 'POST', headers: { 'X-Wishix-Request': '1' }, body, signal: AbortSignal.timeout(25000) })
}
export const deleteWish = id => requestJSON(`/wishes/${encodeURIComponent(id)}`, {
  method: 'DELETE', headers, body: '{}', signal: AbortSignal.timeout(20000),
})

export function uploadWishImage(id, image) {
  const body = new FormData()
  body.append('image', image)
  return requestJSON(`/wishes/${encodeURIComponent(id)}/image`, { method: 'PUT', headers: { 'X-Wishix-Request': '1' }, body, signal: AbortSignal.timeout(25000) })
}
export const deleteWishImage = id => requestJSON(`/wishes/${encodeURIComponent(id)}/image`, { method: 'DELETE', headers, body: '{}', signal: AbortSignal.timeout(20000) })
