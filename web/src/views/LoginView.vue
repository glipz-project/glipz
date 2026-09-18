<script setup lang="ts">
import Button from "../components/ui/Button.vue";
import FormField from "../components/ui/FormField.vue";
import PasswordInput from "../components/ui/PasswordInput.vue";
import Alert from "../components/ui/Alert.vue";
import PageHeader from "../components/ui/PageHeader.vue";
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import AuthLogo from "../components/AuthLogo.vue";
import { api } from "../lib/api";
import { setAccessToken, setMfaToken } from "../auth";
import { safeRelativeRoute } from "../lib/redirect";

const router = useRouter();
const route = useRoute();
const { t } = useI18n();
const email = ref("");
const password = ref("");
const err = ref("");
const loading = ref(false);

async function submit() {
  err.value = "";
  loading.value = true;
  try {
    const res = await api<{
      mfa_required: boolean;
    }>("/api/v1/auth/login", {
      method: "POST",
      json: { email: email.value, password: password.value },
    });
    if (res.mfa_required) {
      setMfaToken("1");
      await router.push("/mfa");
      return;
    }
    setAccessToken();
    const next = safeRelativeRoute(route.query.next, "/feed");
    await router.push(next);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : "";
    err.value = msg === "account_suspended" ? t("auth.login.suspended") : t("auth.login.failed");
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="ui-auth space-y-6">
    <AuthLogo />
    <PageHeader :title="$t('auth.login.title')" :description="$t('auth.login.description')" />
    <form @submit.prevent="submit" :aria-busy="loading">
      <FormField id="login-email" :label="$t('auth.login.email')" required v-slot="field">
        <input v-model="email" :id="field.id" class="ui-input" type="email" required autocomplete="username" :aria-describedby="err ? 'login-error' : undefined" />
      </FormField>
      <FormField id="login-password" :label="$t('auth.login.password')" required v-slot="field">
        <PasswordInput v-model="password" :id="field.id" required autocomplete="current-password" :aria-describedby="err ? 'login-error' : undefined" />
      </FormField>
      <Alert v-if="err" id="login-error" tone="error">{{ err }}</Alert>
      <Button type="submit" :loading="loading" class="w-full">{{ $t(loading ? 'ux.working' : 'auth.login.submit') }}</Button>
    </form>
    <p class="text-center ui-hint">{{ $t('auth.login.firstTime') }} <RouterLink to="/register" class="font-semibold text-lime-700">{{ $t('auth.login.createAccount') }}</RouterLink></p>
  </div>
</template>
