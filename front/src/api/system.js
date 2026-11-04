import { requestJSON } from './client.js'

export async function isServerReady() {
  const data = await requestJSON('/ready', { cache: 'no-store' })
  return data.status === 'ok' && data.service === 'wishix-api'
}
