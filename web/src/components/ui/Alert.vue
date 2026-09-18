<script setup lang="ts">
withDefaults(defineProps<{ tone?: 'info' | 'success' | 'warning' | 'error' }>(), { tone: 'info' });
</script>
<template><div class="ui-alert" :class="`ui-alert--${tone}`" :role="tone === 'error' ? 'alert' : 'status'"><span aria-hidden="true" class="font-bold">{{ tone === 'error' || tone === 'warning' ? '!' : 'i' }}</span><div class="min-w-0"><slot /></div></div></template>
