import { ref } from 'vue'
import { listWishes, deleteWish } from '../api/wishes.js'

export function useWishes() {
  const wishes = ref([])
  const loading = ref(true)
  const error = ref(false)
  async function load() {
    loading.value = true
    error.value = false
    try { wishes.value = (await listWishes()).wishes }
    catch (failure) {
      if (failure.status === 401) window.location.replace('/login')
      else error.value = true
    } finally { loading.value = false }
  }
  async function remove(id) {
    try { await deleteWish(id) }
    catch (failure) {
      if (failure.status === 401) window.location.replace('/login')
      // A wish removed in another tab can safely disappear from this view too.
      if (failure.code !== 'wish_not_found') throw failure
    }
    wishes.value = wishes.value.filter(wish => wish.id !== id)
  }
  return { wishes, loading, error, load, remove }
}
