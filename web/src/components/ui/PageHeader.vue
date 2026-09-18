<script setup lang="ts">
defineProps<{ title: string; description?: string }>();
</script>
<template><header class="ui-page-header"><div class="min-w-0"><h1>{{ title }}</h1><p v-if="description">{{ description }}</p></div><div v-if="$slots.default" class="flex shrink-0 items-center gap-2"><slot /></div></header></template>
