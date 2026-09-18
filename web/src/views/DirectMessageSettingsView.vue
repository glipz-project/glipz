<script setup lang="ts">
import PageHeader from "../components/ui/PageHeader.vue";
import { provide } from "vue";
import DMSettingsPanel from "../components/DMSettingsPanel.vue";
import SettingsBackLink from "../components/SettingsBackLink.vue";
import { securitySettingsKey, useSecuritySettings } from "../composables/useSecuritySettings";

provide(securitySettingsKey, useSecuritySettings());
</script>

<template>
  <div class="w-full px-4 py-8">
    <SettingsBackLink />
    <PageHeader class="mt-4" :title="$t('routes.directMessageSettings')" />
    <div class="mt-6">
      <DMSettingsPanel />
    </div>
  </div>
</template>
