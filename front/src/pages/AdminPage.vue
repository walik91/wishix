<script setup>
import { onMounted, ref } from 'vue'
import { t, locale, languages } from '../i18n.js'
import UiButton from '../components/ui/UiButton.vue'
import TextField from '../components/ui/TextField.vue'
import logo from '../assets/brand/logo-lockup.svg'

const username = ref(''), password = ref(''), busy = ref(false), loading = ref(true), error = ref(''), stats = ref(null)
async function request(path, options = {}) {
  const response = await fetch(`/api/admin/${path}`, { credentials: 'same-origin', cache: 'no-store', ...options })
  if (!response.ok) { const e = new Error(); e.status = response.status; throw e }
  return response.status === 204 ? null : response.json()
}
async function load() {
  loading.value = true; error.value = ''
  try { stats.value = await request('overview') }
  catch (e) { stats.value = null; if (e.status !== 401) error.value = t('admin.unavailable') }
  finally { loading.value = false }
}
async function login() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    await request('login', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }, body: JSON.stringify({ username: username.value, password: password.value }) })
    password.value = ''; await load()
  } catch (e) { password.value = ''; error.value = t(e.status === 401 ? 'admin.invalid' : e.status === 429 ? 'admin.limited' : 'admin.unavailable') }
  finally { busy.value = false }
}
async function logout() {
  busy.value = true; error.value = ''
  try { await request('logout', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Wishix-Request': '1' }, body: '{}' }); stats.value = null; username.value = '' }
  catch { error.value = t('admin.unavailable') }
  finally { busy.value = false }
}
onMounted(load)
</script>
<template>
  <div class="admin-page">
    <header class="admin-header">
      <a href="/" :aria-label="t('admin.home')"><img :src="logo" alt="Wishix" /></a>
      <div class="header-actions">
        <label class="sr-only" for="admin-language">{{ t('admin.language') }}</label>
        <select id="admin-language" v-model="locale"><option v-for="language in languages" :key="language.code" :value="language.code">{{ language.code.toUpperCase() }}</option></select>
        <UiButton v-if="stats" variant="secondary" :loading="busy" @click="logout">{{ t('admin.logout') }}</UiButton>
      </div>
    </header>
    <main class="admin-main">
      <p class="eyebrow">{{ t('admin.eyebrow') }}</p>
      <div v-if="loading" class="panel" role="status">{{ t('admin.loading') }}</div>
      <template v-else-if="stats">
        <h1>{{ t('admin.overview') }}</h1>
        <p class="intro">{{ t('admin.overviewDescription') }}</p>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <section class="stats" :aria-label="t('admin.overview')">
          <article v-for="key in ['users', 'wishes', 'lists', 'reservations']" :key="key" class="stat panel">
            <span class="stat-label">{{ t(`admin.${key}`) }}</span>
            <strong>{{ new Intl.NumberFormat(locale).format(stats[key]) }}</strong>
            <span v-if="key === 'reservations'" class="stat-note">{{ t('admin.reservationNote') }}</span>
          </article>
        </section>
        <UiButton variant="secondary" @click="load">{{ t('admin.refresh') }}</UiButton>
      </template>
      <section v-else class="login panel" aria-labelledby="admin-title">
        <h1 id="admin-title">{{ t('admin.title') }}</h1>
        <p class="intro">{{ t('admin.description') }}</p>
        <form @submit.prevent="login">
          <TextField id="admin-username" v-model="username" :label="t('admin.username')" name="username" autocomplete="username" required maxlength="256" />
          <TextField id="admin-password" v-model="password" :label="t('admin.password')" type="password" name="password" autocomplete="current-password" required maxlength="512" />
          <p v-if="error" class="error" role="alert">{{ error }}</p>
          <UiButton type="submit" :loading="busy">{{ t('admin.signIn') }}</UiButton>
        </form>
      </section>
    </main>
  </div>
</template>
<style scoped>
.admin-page { min-height: 100svh; background: var(--wx-background); }
.admin-header { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 16px clamp(16px, 4vw, 48px); border-bottom: 1px solid var(--wx-border); background: var(--wx-surface); }
.admin-header a { display: flex; align-items: center; min-height: 44px; }
.admin-header img { display: block; width: 130px; max-width: 100%; }
.header-actions { display: flex; align-items: center; gap: 8px; }
select { min-height: 44px; max-width: 120px; padding: 8px; border: 1px solid var(--wx-border); border-radius: var(--wx-radius-sm); color: var(--wx-text); background: var(--wx-surface); }
.admin-main { max-width: 1200px; margin: auto; padding: 40px clamp(16px, 4vw, 48px); }
.eyebrow { color: var(--wx-primary); font-size: var(--wx-text-sm); font-weight: 700; }
h1 { margin: 0; font-size: clamp(24px, 4vw, 32px); line-height: 1.2; overflow-wrap: anywhere; }
.intro { color: var(--wx-text-muted); line-height: 1.6; margin: 12px 0 24px; }
.panel { border: 1px solid var(--wx-border); border-radius: var(--wx-radius-card); background: var(--wx-surface); box-shadow: var(--wx-shadow-card); padding: 24px; }
.login { max-width: 480px; margin: 24px auto; }
form { display: grid; gap: 20px; }
.error { margin: 0; color: var(--wx-error); line-height: 1.5; }
.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 16px; margin-bottom: 24px; }
.stat { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.stat-label { color: var(--wx-text-muted); line-height: 1.4; }
.stat strong { font-size: 32px; overflow-wrap: anywhere; }
.stat-note { color: var(--wx-text-muted); font-size: var(--wx-text-xs); line-height: 1.5; }
@media (max-width: 390px) { .admin-header img { width: 100px; } .admin-header { padding-inline: 12px; } .header-actions { gap: 4px; } select { max-width: 85px; } }
</style>
