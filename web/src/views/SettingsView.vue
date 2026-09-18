<script setup lang="ts">
import type { Ref } from "vue";
import { computed, inject } from "vue";
import { RouterLink } from "vue-router";
import FanclubPatreonSettings from "../components/FanclubPatreonSettings.vue";
import Icon from "../components/Icon.vue";

type AppMe = {
  handle: string;
  is_site_admin?: boolean;
  fanclub_patreon_enabled?: boolean;
} | null;

const appMe = inject<Ref<AppMe> | null>("appMe", null);

const profilePath = computed(() => (appMe?.value?.handle ? `/@${appMe.value.handle}` : "/feed"));
const showAdmin = computed(() => Boolean(appMe?.value?.is_site_admin));
const showFanclubPatreon = computed(() => Boolean(appMe?.value?.fanclub_patreon_enabled));
const showFanclub = computed(() => showFanclubPatreon.value);

const groups = computed(() => [
  { title:'ux.profile', icon:'user' as const, items:[{to:profilePath.value,label:'profileEdit'}] },
  { title:'ux.display', icon:'eye' as const, items:[{to:'/settings/appearance',label:'appearanceSettings'},{to:'/settings/language',label:'languageSettings'}] },
  { title:'ux.posts', icon:'note' as const, items:[{to:'/settings/timeline',label:'timelineSettings'},{to:'/feed/scheduled',label:'scheduledPosts'},{to:'/bookmarks',label:'bookmarks'},{to:'/settings/custom-emojis',label:'customEmojis'}] },
  { title:'ux.communication', icon:'message' as const, items:[{to:'/settings/notifications',label:'notificationSettings'},{to:'/settings/direct-messages',label:'directMessageSettings'}] },
  { title:'ux.security', icon:'lock' as const, items:[{to:'/settings/mfa',label:'mfaSettings'},{to:'/settings/identity-portability',label:'identityPortability'}] },
  { title:'ux.integrations', icon:'settings' as const, items:[{to:'/settings/plugins',label:'pluginSettings'},{to:'/developer/api',label:'apiDeveloper'},{to:'/legal/api-guidelines',label:'apiReferencePublic'}] },
]);
</script>
<template>
  <Teleport to="#app-view-header-slot-desktop"><h1 class="flex min-h-14 items-center text-lg font-bold">{{ $t('views.settings.title') }}</h1></Teleport>
  <Teleport to="#app-view-header-slot-mobile"><h1 class="px-4 py-3 text-xl font-bold">{{ $t('views.settings.title') }}</h1></Teleport>
  <div class="ui-settings-page w-full space-y-8">
    <p class="ui-hint">{{ $t('views.settings.lead') }}</p>
    <section v-for="group in groups" :key="group.title">
      <h2 class="ui-settings-heading"><Icon :name="group.icon" class="h-5 w-5" />{{ $t(group.title) }}</h2>
      <div class="ui-settings-group"><RouterLink v-for="item in group.items" :key="item.to" :to="item.to" class="ui-settings-row"><span>{{ $t('views.settings.items.' + item.label) }}</span><Icon name="chevronDown" class="h-4 w-4 shrink-0 -rotate-90" decorative /></RouterLink></div>
    </section>
    <section v-if="showFanclub"><h2 class="mb-3 text-base font-semibold">{{ $t('views.settings.sections.fanclub') }}</h2><FanclubPatreonSettings v-if="showFanclubPatreon" /></section>
    <section v-if="showAdmin"><h2 class="mb-3 text-base font-semibold">{{ $t('views.settings.sections.admin') }}</h2><div class="ui-settings-group"><RouterLink to="/admin" class="ui-settings-row">{{ $t('views.settings.items.adminControlPanel') }}<Icon name="chevronDown" class="h-4 w-4 -rotate-90" decorative /></RouterLink></div></section>
    <section><h2 class="mb-3 text-base font-semibold">{{ $t('ux.danger') }}</h2><div class="ui-settings-group ui-danger-zone"><RouterLink to="/settings/account-deletion" class="ui-settings-row">{{ $t('views.settings.items.accountDeletion') }}<Icon name="chevronDown" class="h-4 w-4 -rotate-90" decorative /></RouterLink></div></section>
  </div>
</template>
