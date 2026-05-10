<script setup lang="ts">
import { computed, ref } from 'vue'

import { DButton, DCheckbox, DErrorState, DField, DInput } from '@dygo/ui'
import { login, type CurrentUser } from './auth.api'

const identifier = ref('')
const password = ref('')
const remember = ref(false)
const loading = ref(false)
const error = ref('')
const currentUser = ref<CurrentUser | null>(null)

const canSubmit = computed(() => identifier.value.trim() !== '' && password.value !== '')

async function submitLogin() {
  if (!canSubmit.value || loading.value) {
    return
  }

  loading.value = true
  error.value = ''

  try {
    currentUser.value = await login({
      identifier: identifier.value.trim(),
      password: password.value,
      remember: remember.value,
    })
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Sign in failed. Try again.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login-page">
    <section class="login-panel" aria-labelledby="login-title">
      <div class="login-panel__brand">
        <span class="login-panel__mark" aria-hidden="true">d</span>
        <div>
          <p class="login-panel__eyebrow">dygo Studio</p>
          <h1 id="login-title">Sign in</h1>
        </div>
      </div>

      <p class="login-panel__summary">
        Open the workspace for operating records, permissions, activity, and metadata-backed business apps.
      </p>

      <DErrorState v-if="error" :message="error" />

      <div v-if="currentUser" class="login-panel__signed-in" role="status">
        Signed in as {{ currentUser['full-name'] || currentUser.email }}.
      </div>

      <form class="login-form" @submit.prevent="submitLogin">
        <DField id="studio-identifier" label="Email or username">
          <template #default="{ id, invalid }">
            <DInput
              :id="id"
              v-model="identifier"
              name="identifier"
              autocomplete="username"
              placeholder="you@example.com"
              :invalid="invalid"
              :disabled="loading"
              required
            />
          </template>
        </DField>

        <DField id="studio-password" label="Password">
          <template #default="{ id, invalid }">
            <DInput
              :id="id"
              v-model="password"
              name="password"
              type="password"
              autocomplete="current-password"
              :invalid="invalid"
              :disabled="loading"
              required
            />
          </template>
        </DField>

        <label class="login-form__remember">
          <DCheckbox v-model="remember" name="remember" :disabled="loading" />
          <span>Remember this browser</span>
        </label>

        <DButton class="login-form__submit" type="submit" :loading="loading" :disabled="!canSubmit">
          Sign in
        </DButton>
      </form>
    </section>
  </main>
</template>

<style scoped>
.login-page {
  display: grid;
  min-height: 100vh;
  place-items: center;
  background:
    linear-gradient(180deg, oklch(0.985 0.005 246), var(--studio-bg) 38%),
    var(--studio-bg);
  padding: 32px;
}

.login-panel {
  display: grid;
  width: min(100%, 424px);
  gap: 22px;
  border: 1px solid var(--studio-border);
  border-radius: 9px;
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow);
  padding: 28px;
}

.login-panel__brand {
  display: flex;
  align-items: center;
  gap: 13px;
}

.login-panel__mark {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--studio-border);
  border-radius: 8px;
  background: var(--studio-surface-raised);
  color: var(--studio-accent-strong);
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
}

.login-panel__eyebrow {
  margin: 0 0 3px;
  color: var(--studio-text-muted);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.2;
}

.login-panel h1 {
  margin: 0;
  color: var(--studio-text);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.15;
}

.login-panel__summary {
  max-width: 56ch;
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 14px;
  line-height: 1.55;
}

.login-panel__signed-in {
  border: 1px solid oklch(0.52 0.105 156 / 0.24);
  border-radius: 7px;
  background: oklch(0.97 0.018 156);
  color: oklch(0.38 0.095 156);
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.45;
}

.login-form {
  display: grid;
  gap: 16px;
}

.login-form__remember {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 9px;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.3;
}

.login-form__submit {
  width: 100%;
}

@media (max-width: 520px) {
  .login-page {
    align-items: stretch;
    padding: 18px;
  }

  .login-panel {
    align-self: center;
    padding: 22px;
  }
}
</style>
