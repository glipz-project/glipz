<script setup lang="ts">
import PageHeader from "../components/ui/PageHeader.vue";
import { provide } from "vue";
import SecuritySettingsPanel from "../components/SecuritySettingsPanel.vue";
import SettingsBackLink from "../components/SettingsBackLink.vue";
import { securitySettingsKey, useSecuritySettings } from "../composables/useSecuritySettings";

provide(securitySettingsKey, useSecuritySettings());
</script>

<template>
  <div class="w-full px-4 py-8">
    <SettingsBackLink />
    <PageHeader class="mt-4" :title="$t('routes.notificationSettings')" />
    <div class="mt-6">
      <SecuritySettingsPanel section="notifications" />
    </div>
  </div>
</template>
