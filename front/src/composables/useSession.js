import { ref } from 'vue'
import { currentUser, logout } from '../api/auth.js'

const user = ref(null)
const checking = ref(true)
const unavailable = ref(false)
let pending

async function refresh() {
  checking.value = true
  unavailable.value = false
  try { user.value = (await currentUser()).user }
  catch (error) {
    if (error.status === 401) user.value = null
    else unavailable.value = true
  } finally { checking.value = false }
}

async function signOut() {
  await logout()
  user.value = null
  window.location.assign('/login')
}

export function useSession() {
  return { user, checking, unavailable, refresh, signOut, initialize: () => pending ??= refresh() }
}
