<script setup lang="ts">
import PetView from './views/PetView.vue'
import SettingsView from './views/SettingsView.vue'

const view = new URLSearchParams(location.search).get('view') ?? 'pet'
document.documentElement.dataset.view = view
</script>

<template>
  <SettingsView v-if="view === 'settings'" />
  <PetView v-else />
</template>

<style>
html,
body,
#app {
  margin: 0;
  height: 100%;
  font-family: "Microsoft YaHei UI", "PingFang SC", system-ui, sans-serif;
}

html[data-view="pet"],
html[data-view="pet"] body {
  background: transparent;
  overflow: hidden;
  user-select: none;
}
</style>
