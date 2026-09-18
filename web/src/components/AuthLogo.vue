<script setup lang="ts">
import { computed, inject, type Ref } from "vue";
import logoDarkImg from "../assets/glipz-dark.png";
import logoLightImg from "../assets/glipz-light.png";
import { readStoredThemeModePreference, resolveTheme, type ResolvedTheme } from "../lib/theme";

const resolvedTheme = inject<Ref<ResolvedTheme>>("resolvedTheme");
const fallbackTheme = resolveTheme(readStoredThemeModePreference());
const logoImg = computed(() => (resolvedTheme?.value ?? fallbackTheme) === "dark" ? logoDarkImg : logoLightImg);
</script>

<template>
  <div class="ui-auth-logo flex justify-center">
    <img :src="logoImg" alt="Glipz" class="h-16 w-auto object-contain" />
  </div>
</template>
