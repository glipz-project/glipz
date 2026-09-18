<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import Dialog from "./ui/Dialog.vue";
import PostComposerForm from "./PostComposerForm.vue";
import type { PostComposerMode } from "../composables/usePostComposerForm";

const props = defineProps<{
  open: boolean;
  mode: PostComposerMode;
  communityId?: string | null;
  viewerEmail?: string | null;
  viewerHandle?: string | null;
  viewerAvatarUrl?: string | null;
  patreonEnabled?: boolean;
}>();

const emit = defineEmits<{
  "update:open": [value: boolean];
  submitted: [{ mode: PostComposerMode; communityId?: string | null }];
}>();

const { t } = useI18n();
const composerRef = ref<{ focus: () => void } | null>(null);
const isCommunityMode = computed(() => props.mode === "community");
const modalTitle = computed(() =>
  isCommunityMode.value ? t("components.sidebarCompose.communityTitle") : t("components.sidebarCompose.normalTitle"),
);

function close() {
  emit("update:open", false);
}

function onSubmitted(detail: { mode: PostComposerMode; communityId?: string | null }) {
  emit("submitted", detail);
  emit("update:open", false);
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      void nextTick(() => composerRef.value?.focus());
    }
  },
);
</script>

<template>
  <Dialog :open="open" :title="modalTitle" :description="isCommunityMode ? $t('components.sidebarCompose.communityHint') : $t('components.sidebarCompose.normalHint')" @update:open="$emit('update:open', $event)">
    <PostComposerForm ref="composerRef" :mode="mode" :community-id="communityId" :viewer-email="viewerEmail" :viewer-handle="viewerHandle" :viewer-avatar-url="viewerAvatarUrl" :patreon-enabled="!!patreonEnabled" layout="modal" @submitted="onSubmitted" />
  </Dialog>
</template>
