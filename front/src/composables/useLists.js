import { ref } from 'vue'
import { listLists, getList } from '../api/lists.js'

export function listFailure(error) {
  if (error.status === 401) window.location.replace('/login')
  return ['list_title_invalid', 'list_description_invalid', 'list_date_invalid', 'list_visibility_invalid', 'list_selection_invalid', 'list_changed', 'list_not_found'].includes(error.code) ? error.code : 'unavailable'
}
export function useLists() {
  const lists = ref([]), loading = ref(true), error = ref('')
  async function load() {
    loading.value = true; error.value = ''
    try { lists.value = (await listLists()).lists }
    catch (failure) { error.value = listFailure(failure) }
    finally { loading.value = false }
  }
  return { lists, loading, error, load }
}
export function useList(id) {
  const list = ref(null), wishes = ref([]), loading = ref(true), error = ref('')
  async function load() {
    loading.value = true; error.value = ''
    try { const data = await getList(id); list.value = data.list; wishes.value = data.wishes }
    catch (failure) { error.value = listFailure(failure) }
    finally { loading.value = false }
  }
  return { list, wishes, loading, error, load }
}
