<script setup lang="ts">
import Button from './Button.vue';
defineProps<{ title: string; description?: string; actionLabel?: string; busy?: boolean }>();
defineEmits<{ action: [] }>();
</script>
<template><div class="ui-empty" role="status"><span v-if="busy" class="ui-spinner" aria-hidden="true" /><h2>{{ title }}</h2><p v-if="description">{{ description }}</p><Button v-if="actionLabel" variant="secondary" @click="$emit('action')">{{ actionLabel }}</Button><slot /></div></template>
