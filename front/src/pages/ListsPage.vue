<script setup>
import { ref, watch, watchEffect } from 'vue'
import { t } from '../i18n.js'
import { useSession } from '../composables/useSession.js'
import { useLists } from '../composables/useLists.js'
import UiButton from '../components/ui/UiButton.vue'
import UiCard from '../components/ui/UiCard.vue'
import UiSkeleton from '../components/ui/UiSkeleton.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import AppIcon from '../components/ui/AppIcon.vue'
import UiToast from '../components/ui/UiToast.vue'
import ListCard from '../components/wishes/ListCard.vue'
const { user, checking, unavailable, refresh } = useSession()
const { lists, loading, error, load } = useLists()
const notice = ref(new URLSearchParams(window.location.search).get('notice') === 'deleted')
if (notice.value) window.history.replaceState(null, '', window.location.pathname)
watch([user, checking, unavailable], ([account, pending, failed]) => { if (account && !pending && !failed) load() }, { immediate: true })
watchEffect(() => { document.title = `${t('lists.mine')} · ${t('appName')}` })
async function retry() { await refresh(); if (!user.value && !unavailable.value) window.location.replace('/login') }
</script>
<template>
  <div class="lists-page" :aria-busy="checking || loading">
    <UiToast v-if="notice" @dismiss="notice = false">{{ t('lists.notice.deleted') }}</UiToast>
    <header class="list-page-heading"><div><p class="list-eyebrow">{{ t('auth.my') }}</p><h1>{{ t('lists.mine') }} <span v-if="!checking && !loading && !error && !unavailable" class="list-count">{{ lists.length }}</span></h1><p class="list-muted">{{ t('lists.description') }}</p></div><UiButton href="/my/lists/new"><AppIcon name="plus" />{{ t('lists.create') }}</UiButton></header>
    <div v-if="checking || (user && loading)" class="list-grid" role="status" :aria-label="t('lists.loading')"><UiSkeleton v-for="i in 3" :key="i" class="list-skeleton" /></div>
    <UiCard v-else-if="unavailable || error" class="list-empty"><EmptyState icon="globe" :title="t('lists.loadError')" :description="t('lists.error.unavailable')"><UiButton @click="unavailable ? retry() : load()">{{ t('checkAgain') }}</UiButton></EmptyState></UiCard>
    <template v-else-if="user"><div v-if="lists.length" class="list-grid"><ListCard v-for="list in lists" :key="list.id" :list="list" /></div><UiCard v-else class="list-empty"><EmptyState icon="list" :title="t('lists.emptyTitle')" :description="t('lists.emptyDescription')"><UiButton href="/my/lists/new"><AppIcon name="plus" />{{ t('lists.create') }}</UiButton></EmptyState></UiCard></template>
  </div>
</template>
