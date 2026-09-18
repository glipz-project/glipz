<script setup lang="ts">
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
defineOptions({ inheritAttrs: false });
const model = defineModel<string>();
const visible = ref(false);
const { t } = useI18n();
</script>
<template><div class="ui-password"><input v-bind="$attrs" v-model="model" :type="visible ? 'text' : 'password'" class="ui-input" /><button type="button" :aria-pressed="visible" @click="visible = !visible">{{ t(visible ? 'ux.hidePassword' : 'ux.showPassword') }}</button></div></template>
