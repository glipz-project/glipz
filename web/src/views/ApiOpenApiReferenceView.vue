<script setup lang="ts">
import { ApiReference } from "@scalar/api-reference";
import "@scalar/api-reference/style.css";
import { RouterLink } from "vue-router";
import { useI18n } from "vue-i18n";
import { useBackLink } from "../lib/useBackLink";

const { t } = useI18n();
const { to: backTo, label: backLabel, onClick: backOnClick } = useBackLink({ fallbackTo: "/register" });

const scalarConfiguration = {
  url: "/openapi.yaml",
  withDefaultFonts: false,
  agent: { disabled: true },
};
</script>

<template>
  <div class="w-full min-w-0 px-4 py-8 text-neutral-900 sm:px-6 lg:px-8">
    <div class="mx-auto flex w-full max-w-6xl flex-col gap-4">
      <RouterLink :to="backTo" class="text-sm font-medium text-lime-700 hover:text-lime-800" @click="backOnClick">
        {{ backLabel }}
      </RouterLink>
      <p class="text-sm leading-6 text-neutral-600">
        {{ t("views.apiOpenApi.languageNote") }}
      </p>
      <div class="isolate min-h-[70vh] w-full min-w-0 overflow-hidden rounded-xl border border-neutral-200 bg-white">
        <ApiReference :configuration="scalarConfiguration" />
      </div>
    </div>
  </div>
</template>
