<script setup>
import { computed } from 'vue'
import { t } from '../../i18n.js'
import { rememberProfileDestination } from '../../composables/usePublicProfileFocus.js'

const props = defineProps({
  username: { type: String, required: true },
  current: { type: String, required: true },
  showFollowers: Boolean,
  showFollowing: Boolean,
})

const sections = computed(() => {
  const profile = `/u/${encodeURIComponent(props.username)}`
  const links = [{ id: 'lists', href: profile, label: 'social.wishLists' }]
  if (props.showFollowers) links.push({ id: 'followers', href: `${profile}/followers`, label: 'social.followers' })
  if (props.showFollowing) links.push({ id: 'following', href: `${profile}/following`, label: 'social.following' })
  return links
})
</script>

<template>
  <nav class="profile-navigation" :aria-label="t('social.profileNavigation')">
    <a
      v-for="section in sections"
      :key="section.id"
      :href="section.href"
      :aria-current="section.id === current ? 'page' : undefined"
      @click="rememberProfileDestination"
    >{{ t(section.label) }}</a>
  </nav>
</template>

<style scoped>
.profile-navigation {
  display: flex;
  flex-wrap: wrap;
  gap: var(--wx-space-1);
}
.profile-navigation a {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  min-width: 44px;
  max-width: 100%;
  padding: var(--wx-space-1) var(--wx-space-2);
  border-radius: var(--wx-radius-md);
  color: var(--wx-text);
  font-size: var(--wx-text-sm);
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
  text-decoration: none;
}
.profile-navigation a:hover,
.profile-navigation a:active,
.profile-navigation a[aria-current='page'] {
  background: var(--wx-lavender);
  color: var(--wx-primary-pressed);
}
.profile-navigation a[aria-current='page'] {
  font-weight: 700;
  text-decoration: underline;
  text-underline-offset: .2em;
}
</style>
