<script setup lang="ts">
import Dialog from "./ui/Dialog.vue";
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import type { TimelinePost } from "../types/timeline";

const props = withDefaults(
  defineProps<{
    open: boolean;
    post: TimelinePost | null;
    /** Disable interactions while the API request is in flight. */
    submitting?: boolean;
  }>(),
  { submitting: false },
);

const emit = defineEmits<{
  "update:open": [value: boolean];
  plain: [];
  "with-comment": [text: string];
}>();
const { t } = useI18n();

const step = ref<"choose" | "comment">("choose");
const commentText = ref("");
const emptyHint = ref(false);

watch(
  () => props.open,
  (v) => {
    if (v) {
      step.value = "choose";
      commentText.value = "";
      emptyHint.value = false;
    }
  },
);

function close() {
  if (props.submitting) return;
  emit("update:open", false);
}

function previewCaption(p: TimelinePost): string {
  const c = (p.caption ?? "").replace(/\s+/g, " ").trim();
  if (!c) return t("components.repostModal.emptyBody");
  return c.length > 160 ? `${c.slice(0, 160)}…` : c;
}

function onPlain() {
  emit("plain");
}

function goComment() {
  step.value = "comment";
  emptyHint.value = false;
}

function backToChoose() {
  step.value = "choose";
  emptyHint.value = false;
}

function submitWithComment() {
  const t = commentText.value.trim();
  if (!t) {
    emptyHint.value = true;
    return;
  }
  emit("with-comment", t);
}
</script>

<template>
  <Dialog :open="open && !!post" :title="$t('components.repostModal.title')" :description="post ? previewCaption(post) : ''" @update:open="close">
        <div v-if="step === 'choose'" class="flex flex-col gap-2 px-4 py-4">
          <p class="text-sm text-neutral-600">{{ $t("components.repostModal.description") }}</p>
          <button
            type="button"
            class="w-full rounded-xl bg-lime-600 py-3 text-sm font-semibold text-white hover:bg-lime-700 disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="props.submitting"
            @click="onPlain"
          >
            {{ $t("components.repostModal.plain") }}
          </button>
          <button
            type="button"
            class="w-full rounded-xl border border-neutral-200 py-3 text-sm font-medium text-neutral-800 hover:bg-neutral-50 disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="props.submitting"
            @click="goComment"
          >
            {{ $t("components.repostModal.withComment") }}
          </button>
          <button
            type="button"
            class="py-2 text-sm text-neutral-500 hover:text-neutral-800 disabled:opacity-50"
            :disabled="props.submitting"
            @click="close"
          >
            {{ $t("components.repostModal.cancel") }}
          </button>
        </div>

        <div v-else class="flex flex-col gap-3 px-4 py-4">
          <label class="text-sm font-medium text-neutral-800" for="repost-comment-input">{{ $t("components.repostModal.comment") }}</label>
          <textarea
            id="repost-comment-input"
            v-model="commentText"
            rows="4"
            maxlength="2000"
            class="w-full resize-y rounded-xl border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none ring-lime-500 focus:ring-2"
            :placeholder="$t('components.repostModal.commentPlaceholder')"
            @input="emptyHint = false"
          />
          <p v-if="emptyHint" class="text-xs text-red-600">{{ $t("components.repostModal.commentRequired") }}</p>
          <p class="text-xs text-neutral-500">{{ $t("components.repostModal.maxLength") }}</p>
          <div class="flex flex-col gap-2 sm:flex-row-reverse sm:justify-end">
            <button
              type="button"
              class="w-full rounded-xl bg-lime-600 py-3 text-sm font-semibold text-white hover:bg-lime-700 disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto sm:min-w-[8rem] sm:px-4"
              :disabled="props.submitting"
              @click="submitWithComment"
            >
              {{ $t("components.repostModal.submit") }}
            </button>
            <button
              type="button"
              class="w-full rounded-xl border border-neutral-200 py-3 text-sm font-medium text-neutral-800 hover:bg-neutral-50 disabled:opacity-50 sm:w-auto sm:px-4"
              :disabled="props.submitting"
              @click="backToChoose"
            >
              {{ $t("components.repostModal.back") }}
            </button>
          </div>
          <button
            type="button"
            class="text-sm text-neutral-500 hover:text-neutral-800 disabled:opacity-50"
            :disabled="props.submitting"
            @click="close"
          >
            {{ $t("components.repostModal.cancel") }}
          </button>
        </div>
  </Dialog>
</template>
