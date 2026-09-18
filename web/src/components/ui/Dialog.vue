<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import IconButton from './IconButton.vue';
import Icon from '../Icon.vue';
const props = defineProps<{ open: boolean; title: string; description?: string }>();
const emit = defineEmits<{ 'update:open': [value: boolean] }>();
const dialog = ref<HTMLDialogElement>();
const { t } = useI18n();
let origin: HTMLElement | null = null;
function close() { emit('update:open', false); }
watch(() => props.open, async open => {
  if (open) {
    origin = document.activeElement as HTMLElement | null;
    await nextTick();
    dialog.value?.showModal();
  } else {
    dialog.value?.close();
    if (origin?.isConnected) origin.focus();
  }
}, { immediate: true });
onBeforeUnmount(() => { dialog.value?.close(); if (origin?.isConnected) origin.focus(); });
</script>
<template>
  <Teleport to="body"><dialog ref="dialog" class="ui-dialog" :aria-label="title" @cancel.prevent="close" @click="event => { if (event.target === dialog) { const r = dialog.getBoundingClientRect(); if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) close(); } }">
    <header class="ui-dialog-header"><div><h2>{{ title }}</h2><p v-if="description" class="ui-hint">{{ description }}</p></div><IconButton :label="t('views.feed.lightboxClose')" @click="close"><Icon name="close" class="h-5 w-5" /></IconButton></header>
    <slot v-if="open" />
  </dialog></Teleport>
</template>
