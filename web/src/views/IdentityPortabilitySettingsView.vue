<script setup lang="ts">
import PageHeader from "../components/ui/PageHeader.vue";
import IdentityPortabilitySettings from "../components/IdentityPortabilitySettings.vue";
import SettingsBackLink from "../components/SettingsBackLink.vue";
</script>

<template>
  <div class="w-full px-4 py-8">
    <SettingsBackLink />
    <PageHeader class="mt-4" :title="$t('routes.identityPortabilitySettings')" />
    <div class="mt-6">
      <IdentityPortabilitySettings />
    </div>
  </div>
</template>
