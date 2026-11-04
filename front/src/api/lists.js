import { requestJSON } from './client.js'
const headers = { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }
const idPath = id => `/lists/${encodeURIComponent(id)}`
const mutate = (path, method, data) => requestJSON(path, { method, headers, body: JSON.stringify(data), signal: AbortSignal.timeout(20000) })
export const listLists = () => requestJSON('/lists')
export const getList = id => requestJSON(idPath(id))
export const createList = input => mutate('/lists', 'POST', input)
export const updateList = (id, input) => mutate(idPath(id), 'PUT', input)
export const deleteList = (id, revision) => mutate(idPath(id), 'DELETE', { revision })
export const clearList = (id, revision) => mutate(`${idPath(id)}/wishes`, 'DELETE', { revision })
export const copyList = (id, title) => mutate(`${idPath(id)}/copy`, 'POST', { title })
export const rotateListLink = (id, revision) => mutate(`${idPath(id)}/share-link`, 'POST', { revision })
export const addListWishes = (id, wish_ids) => mutate(`${idPath(id)}/wishes`, 'POST', { wish_ids })
export const removeListWish = (id, wishID) => mutate(`${idPath(id)}/wishes/${encodeURIComponent(wishID)}`, 'DELETE', {})
export const getWishLists = id => requestJSON(`/wishes/${encodeURIComponent(id)}/lists`)
export const setWishLists = (id, list_ids, version) => mutate(`/wishes/${encodeURIComponent(id)}/lists`, 'PUT', { list_ids, version })
export const viewSharedList = token => mutate('/public/lists/view', 'POST', { token })
export const publicProfile = username => requestJSON(`/public/profiles/${encodeURIComponent(username)}`)
export const getCollectionSetting = () => requestJSON('/profile/collection')
export const setCollectionSetting = show_all_wishes => mutate('/profile/collection', 'PUT', { show_all_wishes })
export const publicCollection = username => requestJSON(`/public/profiles/${encodeURIComponent(username)}/wishes`)
export async function publicCollectionImage(username, wishID, signal) {
  const response = await fetch(`/api/public/profiles/${encodeURIComponent(username)}/wishes/${encodeURIComponent(wishID)}/image`, { credentials: 'omit', signal })
  if (!response.ok || response.headers.get('content-type') !== 'image/jpeg') throw new Error('Image unavailable')
  return response.blob()
}
export async function sharedWishImage(token, wish_id, signal) {
  const response = await fetch('/api/public/lists/image', { method: 'POST', headers, body: JSON.stringify({ token, wish_id }), credentials: 'omit', signal })
  if (!response.ok || response.headers.get('content-type') !== 'image/jpeg') throw new Error('Image unavailable')
  return response.blob()
}
