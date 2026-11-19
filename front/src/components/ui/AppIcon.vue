<script setup>
defineProps({ name: { type: String, required: true } })

const paths = {
  more: ['M5 12h.01', 'M12 12h.01', 'M19 12h.01'],
  home: ['m3 10 9-7 9 7', 'M5 9v12h14V9', 'M9 21v-8h6v8'],
  search: ['M20 20l-5-5', 'M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0'],
  users: ['M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2', 'M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0', 'M22 21v-2a4 4 0 0 0-3-3.87', 'M16 3.13a4 4 0 0 1 0 7.75'],
  user: ['M20 21v-2a7 7 0 0 0-14 0v2', 'M17 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0'],
  plus: ['M12 5v14', 'M5 12h14'],
  image: ['M3 3h18v18H3z', 'm3 17 6-6 4 4 3-3 5 5', 'M16 7h.01'],
  download: ['M12 3v12', 'm7 10 5 5 5-5', 'M4 17v3h16v-3'],
  trash: ['M3 6h18', 'M9 6V3h6v3', 'M5 6l1 15h12l1-15', 'M10 10v7', 'M14 10v7'],
  link: ['M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-2 2', 'M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l2-2'],
  copy: ['M8 8h12v12H8z', 'M4 16V4h12'],
  refresh: ['M20 7v5h-5', 'M4 17v-5h5', 'M5 9a7 7 0 0 1 12-3l3 6', 'M19 15a7 7 0 0 1-12 3l-3-6'],
  calendar: ['M4 5h16v16H4z', 'M8 3v4', 'M16 3v4', 'M4 11h16', 'M8 15h2', 'M14 15h2'],
  close: ['m6 6 12 12', 'M18 6 6 18'],
  logout: ['M9 3H4v18h5', 'M9 12h12', 'm17 8 4 4-4 4'],
  globe: ['M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0', 'M3 12h18', 'M12 3a19 19 0 0 1 0 18 19 19 0 0 1 0-18'],
  chevron: ['m8 10 4 4 4-4'],
  arrow: ['M5 12h14', 'm13 6 6 6-6 6'],
  eye: ['M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12', 'M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0'],
  eyeOff: ['m3 3 18 18', 'M10.6 5.1 12 5c6.5 0 10 7 10 7a19 19 0 0 1-3 3.9', 'M6.5 6.5A21 21 0 0 0 2 12s3.5 7 10 7c1.8 0 3.4-.5 4.8-1.3', 'M10 10a3 3 0 0 0 4 4'],
  check: ['m5 12 4 4L19 6'],
  mail: ['M4 4h16a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1', 'm3 5 9 7 9-7'],
  list: ['M9 6h12', 'M9 12h12', 'M9 18h12', 'M3 6h.01', 'M3 12h.01', 'M3 18h.01'],
  share: ['M18 5a2 2 0 1 1 4 0 2 2 0 0 1-4 0', 'M2 12a2 2 0 1 1 4 0 2 2 0 0 1-4 0', 'M18 19a2 2 0 1 1 4 0 2 2 0 0 1-4 0', 'm6 11 12-5', 'm6 13 12 5'],
  lock: ['M5 10h14v11H5z', 'M8 10V7a4 4 0 0 1 8 0v3', 'M12 14v3'],
  heart: ['M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8'],
}
</script>

<template>
  <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
    <path v-for="(path, index) in paths[name]" :key="index" :d="path" />
  </svg>
</template>

<style scoped>
.icon { width: 20px; height: 20px; flex: 0 0 auto; }
</style>
