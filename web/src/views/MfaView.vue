<script setup lang="ts">
import Button from "../components/ui/Button.vue";
import FormField from "../components/ui/FormField.vue";
import PasswordInput from "../components/ui/PasswordInput.vue";
import Alert from "../components/ui/Alert.vue";
import PageHeader from "../components/ui/PageHeader.vue";
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import AuthLogo from "../components/AuthLogo.vue";
import { api } from "../lib/api";
import { clearTokens, getMfaToken, setAccessToken } from "../auth";

const router = useRouter();
const { t } = useI18n();
const code = ref("");
const err = ref("");
const loading = ref(false);

onMounted(() => {
  if (!getMfaToken()) {
    router.replace("/login");
  }
});

async function submit() {
  err.value = "";
  const mfa = getMfaToken();
  if (!mfa) {
    await router.replace("/login");
    return;
  }
  loading.value = true;
  try {
    await api<{ status: string }>("/api/v1/auth/mfa/verify", {
      method: "POST",
      json: { code: code.value },
    });
    clearTokens();
    setAccessToken();
    await router.push("/feed");
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : "";
    err.value = msg === "account_suspended" ? t("auth.mfa.suspended") : t("auth.mfa.failed");
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div class="ui-auth space-y-6">
    <AuthLogo />
    <PageHeader :title="$t('auth.mfa.title')" :description="$t('auth.mfa.description')" />
    <form @submit.prevent="submit" :aria-busy="loading">
      <FormField id="mfa-code" :label="$t('auth.mfa.code')" :error="err" required v-slot="field">
        <input v-model="code" :id="field.id" :aria-describedby="field.describedby" :aria-invalid="field.invalid" class="ui-input" type="text" inputmode="numeric" pattern="[0-9]{6,8}" minlength="6" maxlength="8" required autocomplete="one-time-code" />
      </FormField>
      <Button type="submit" :loading="loading" class="w-full">{{ $t(loading ? 'ux.working' : 'auth.mfa.submit') }}</Button>
    </form>
  </div>
</template>
