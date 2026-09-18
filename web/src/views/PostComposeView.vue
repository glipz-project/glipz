<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PostComposerForm from "../components/PostComposerForm.vue";
import IconButton from "../components/ui/IconButton.vue";
import Icon from "../components/Icon.vue";
import { api } from "../lib/api";
import { parseComposerReplyQuery } from "../lib/postComposer";
import type { PostComposerReplyTarget } from "../composables/usePostComposerForm";
const route = useRoute();
const router = useRouter();
const me = ref<{email:string; handle:string; avatar_url:string|null; fanclub_patreon_enabled?:boolean} | null>(null);
const composer = ref<{startReply:(target:PostComposerReplyTarget)=>void; cancelReply:()=>void} | null>(null);
async function syncReply() {
  await nextTick();
  const target = parseComposerReplyQuery(route.query);
  if (target) composer.value?.startReply({...target,user_email:''});
  else composer.value?.cancelReply();
}
onMounted(async () => {
  void syncReply();
  try { me.value = await api('/api/v1/me'); } catch { /* The route guard handles expired authentication. */ }
});
watch(() => route.fullPath, syncReply);
</script>
<template>
  <Teleport defer to="#app-view-header-slot-desktop">
    <div class="flex min-h-14 items-center gap-3 px-2"><IconButton :label="$t('views.compose.back')" @click="router.push('/feed')"><Icon name="back" class="h-5 w-5" /></IconButton><h1 class="text-lg font-bold">{{ $t('views.compose.title') }}</h1></div>
  </Teleport>
  <Teleport defer to="#app-view-header-slot-mobile">
    <div class="flex min-h-14 items-center gap-3 px-2"><IconButton :label="$t('views.compose.back')" @click="router.push('/feed')"><Icon name="back" class="h-5 w-5" /></IconButton><h1 class="text-lg font-bold">{{ $t('views.compose.title') }}</h1></div>
  </Teleport>
  <div class="w-full px-4 py-4"><PostComposerForm ref="composer" layout="embedded" :viewer-email="me?.email" :viewer-handle="me?.handle" :viewer-avatar-url="me?.avatar_url" :patreon-enabled="!!me?.fanclub_patreon_enabled" @reply-cancelled="router.replace('/compose')" @submitted="result => router.push(result.scheduled ? '/feed/scheduled' : `/posts/${result.id}`)" /></div>
</template>
