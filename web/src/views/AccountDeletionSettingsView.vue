<script setup lang="ts">
import PageHeader from "../components/ui/PageHeader.vue";
import AccountDeletionSettings from "../components/AccountDeletionSettings.vue";
import SettingsBackLink from "../components/SettingsBackLink.vue";
</script>

<template>
  <div class="w-full px-4 py-8">
    <SettingsBackLink />
    <PageHeader class="mt-4" :title="$t('routes.accountDeletionSettings')" />
    <div class="mt-6">
      <AccountDeletionSettings />
    </div>
  </div>
</template>
