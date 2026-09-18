<script setup lang="ts">
import Button from './Button.vue';
withDefaults(defineProps<{ label: string; disabled?: boolean; loading?: boolean; pressed?: boolean }>(), { disabled: false, pressed: undefined });
</script>
<template><Button variant="ghost" class="ui-icon-button" :aria-label="label" :title="label" :aria-pressed="pressed" :disabled="disabled" :loading="loading"><slot /></Button></template>
