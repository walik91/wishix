<script setup>
import { ref, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { trackEvent } from '../analytics.js'
import { useSession } from '../composables/useSession.js'
import { listFailure } from '../composables/useLists.js'
import { createList } from '../api/lists.js'
import ListForm from '../components/lists/ListForm.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiButton from '../components/ui/UiButton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
const { user, checking, unavailable, refresh } = useSession()
const busy = ref(false), error = ref('')
watchEffect(() => { document.title = `${t('lists.create')} · ${t('appName')}` })
async function save(input) { if (busy.value) return; busy.value = true; error.value = ''; try { const { list } = await createList(input); trackEvent('list_created'); window.location.assign(`/my/lists/${list.id}?notice=created`) } catch (failure) { error.value = listFailure(failure); busy.value = false } }
async function retry() { await refresh(); if (!user.value && !unavailable.value) window.location.replace('/login') }
function cancel() { if (!busy.value) window.location.assign('/my/lists') }
</script>
<template>
  <div class="lists-page list-narrow">
    <UiButton href="/my/lists" variant="secondary" :disabled="busy">{{ t('lists.back') }}</UiButton>
    <header class="list-page-heading list-form-heading"><div><h1>{{ t('lists.create') }}</h1><p class="list-muted">{{ t('lists.createDescription') }}</p></div></header>
    <UiSkeleton v-if="checking" class="list-form-skeleton" />
    <UiCard v-else-if="unavailable" class="list-empty"><EmptyState icon="globe" :title="t('lists.loadError')"><UiButton @click="retry">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <UiCard v-else-if="user" class="list-form-card"><ListForm :busy="busy" :error="error" @submit="save" @cancel="cancel" /></UiCard>
  </div>
</template>
