import { onMounted, ref } from 'vue'
import { isServerReady } from '../api/system.js'

export function useServerConnection() {
  const status = ref('checking')

  async function checkConnection() {
    status.value = 'checking'

    try {
      status.value = await isServerReady() ? 'connected' : 'unavailable'
    } catch {
      status.value = 'unavailable'
    }
  }

  onMounted(checkConnection)

  return { status, checkConnection }
}
