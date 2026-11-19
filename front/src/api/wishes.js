import { requestJSON } from './client.js'

const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
export const listWishes = () => requestJSON('/wishes')
export const getWish = id => requestJSON(`/wishes/${encodeURIComponent(id)}`)
export const copyWish = id => requestJSON(`/wishes/${encodeURIComponent(id)}/copy`, {
  method: 'POST', headers, body: '{}', signal: AbortSignal.timeout(20000),
})
export const updateWish = (id, data) => requestJSON(`/wishes/${encodeURIComponent(id)}`, {
  method: 'PUT', headers, body: JSON.stringify(data), signal: AbortSignal.timeout(20000),
})
export const previewWish = (url, locale, signal) => requestJSON('/wishes/import-preview', {
  method: 'POST', headers, body: JSON.stringify({ url, locale }),
  signal: AbortSignal.any([signal, AbortSignal.timeout(20000)]),
})
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
