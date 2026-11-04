<script setup>
import { useServerConnection } from '../composables/useServerConnection.js'
import { t } from '../i18n.js'

const { status, checkConnection } = useServerConnection()
</script>

<template>
  <section class="page">
    <p class="eyebrow">{{ t('stage') }}</p>
    <h1>{{ t('title') }}</h1>
    <p class="intro">{{ t('intro') }}</p>

    <section class="connection" aria-labelledby="connection-title">
      <h2 id="connection-title">{{ t('statusTitle') }}</h2>
      <p class="status" :class="status" role="status">
        <span class="status-dot" aria-hidden="true"></span>
        {{ t(status) }}
      </p>
      <button type="button" :disabled="status === 'checking'" @click="checkConnection">
        {{ t('checkAgain') }}
      </button>
    </section>

    <p class="placeholder">{{ t('placeholder') }}</p>
  </section>
</template>

<style scoped>
.page { max-width: 43rem; padding-block: var(--wx-space-10); }
.eyebrow { margin: 0 0 1rem; color: var(--wx-text-muted); font-size: 0.8rem; font-weight: 650; letter-spacing: 0.12em; text-transform: uppercase; }
h1 { margin: 0; font-size: clamp(2.5rem, 7vw, 4.25rem); font-weight: 650; line-height: 1.1; letter-spacing: -0.055em; }
.intro { margin: 1.5rem 0 2rem; color: var(--wx-text-muted); font-size: 1.1rem; line-height: 1.7; }

.connection { padding: 1.5rem; background: var(--wx-surface); border: 1px solid var(--wx-border); border-radius: 1.25rem; }
h2 { margin: 0; font-size: 1.1rem; line-height: 1.5; font-weight: 650; }
.status { display: flex; align-items: center; gap: 0.7rem; margin: 1rem 0 1.5rem; line-height: 1.6; }
.status-dot { flex: 0 0 0.6rem; height: 0.6rem; border-radius: 50%; background: var(--wx-text-muted); }
.connected .status-dot { background: var(--wx-success); }
.unavailable .status-dot { background: var(--wx-error); }
button { min-height: 2.75rem; padding: 0.65rem 1.1rem; border: 0; border-radius: 0.7rem; background: var(--wx-primary); color: white; cursor: pointer; }
button:hover:enabled { background: var(--wx-primary-hover); }
button:active:enabled { background: var(--wx-primary-pressed); }
button:disabled { opacity: 0.6; cursor: wait; }
.placeholder { margin-top: 1.5rem; color: var(--wx-text-muted); font-size: 0.9rem; line-height: 1.7; }

@media (min-width: 48rem) {
  .page { padding-block: calc(var(--wx-space-10) * 2); }
  .connection { padding: 2rem; }
}
</style>
