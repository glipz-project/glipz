<script setup lang="ts">
defineProps<{ id: string; label: string; hint?: string; error?: string; required?: boolean }>();
</script>
<template>
  <div class="ui-field">
    <label :for="id" class="ui-label">{{ label }}<span v-if="required" aria-hidden="true"> *</span></label>
    <slot :id="id" :describedby="[hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined" :invalid="!!error" />
    <p v-if="hint" :id="`${id}-hint`" class="ui-hint">{{ hint }}</p>
    <p v-if="error" :id="`${id}-error`" class="ui-field-error" role="alert">{{ error }}</p>
  </div>
</template>
