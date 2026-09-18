<script setup lang="ts">
import PageHeader from "../components/ui/PageHeader.vue";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import SettingsBackLink from "../components/SettingsBackLink.vue";
import { setLocale, supportedLocaleOptions, type AppLocale } from "../i18n";

const saved = ref(false);
const { t, locale } = useI18n();

const localeOptions = computed(() =>
  supportedLocaleOptions.map((option) => ({
    value: option.value,
    label: t(option.labelKey),
  })),
);

function selectLocale(next: AppLocale) {
  saved.value = true;
  setLocale(next);
}
</script>

<template>
  <div class="w-full px-4 py-8">
    <SettingsBackLink />
    <PageHeader class="mt-4" :title="$t('routes.languageSettings')" />

    <p v-if="saved" role="status" class="ui-hint">{{ $t('ux.saved') }}</p>
    <section class="mt-6">
      <h2 class="text-xs font-semibold uppercase tracking-wide text-neutral-500">
        {{ $t("app.locale.heading") }}
      </h2>
      <div class="mt-3 rounded-2xl border border-neutral-200 bg-white p-4 shadow-sm">
        <label class="block text-sm">
          <span class="font-medium text-neutral-900">{{ $t("app.locale.heading") }}</span>
          <select
            class="mt-2 w-full rounded-xl border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none ring-lime-500/30 transition focus:border-lime-400 focus:ring-2 focus:ring-lime-400/40"
            :value="locale"
            @change="selectLocale(($event.target as HTMLSelectElement).value as AppLocale)"
          >
            <option v-for="opt in localeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>
      </div>
    </section>
  </div>
</template>
