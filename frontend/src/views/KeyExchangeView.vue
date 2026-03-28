<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="relative z-20 px-4 py-4 sm:px-6">
      <nav class="mx-auto flex max-w-5xl items-center justify-between">
        <router-link to="/home" class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-lg font-semibold tracking-tight">{{ siteName }}</span>
        </router-link>
        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
        </div>
      </nav>
    </header>

    <main class="mx-auto flex w-full max-w-5xl flex-1 flex-col px-4 pb-12 pt-8 sm:px-6">
      <section class="mx-auto w-full max-w-3xl text-center">
        <h1 class="text-3xl font-bold tracking-tight sm:text-4xl">
          {{ t('keyExchange.title') }}
        </h1>
        <p class="mx-auto mt-3 max-w-2xl text-sm text-gray-500 dark:text-dark-400 sm:text-base">
          {{ t('keyExchange.subtitle') }}
        </p>
      </section>

      <section class="mx-auto mt-10 w-full max-w-3xl rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-8">
        <form class="space-y-4" @submit.prevent="handleResolve">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">
              {{ t('keyExchange.codeLabel') }}
            </label>
            <div class="flex flex-col gap-3 sm:flex-row">
              <input
                v-model="code"
                type="text"
                autocapitalize="characters"
                spellcheck="false"
                :placeholder="t('keyExchange.placeholder')"
                class="w-full rounded-2xl border border-gray-200 bg-white px-4 py-3 text-base uppercase tracking-[0.2em] outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-200 dark:border-dark-700 dark:bg-dark-950 dark:focus:ring-primary-900"
              />
              <button
                type="submit"
                :disabled="resolving"
                class="inline-flex min-w-[180px] items-center justify-center rounded-2xl bg-primary-500 px-5 py-3 text-sm font-medium text-white transition hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <Icon v-if="!resolving" name="key" size="sm" class="mr-2" />
                <svg v-else class="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
                  <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" opacity="0.25" />
                  <path d="M12 2a10 10 0 0 1 10 10" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
                </svg>
                {{ resolving ? t('keyExchange.resolving') : t('keyExchange.button') }}
              </button>
            </div>
          </div>

          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('keyExchange.hint') }}
          </p>
        </form>
      </section>

      <section
        v-if="result"
        class="mx-auto mt-6 grid w-full max-w-3xl gap-4"
      >
        <div class="rounded-3xl border border-emerald-200 bg-emerald-50 p-5 dark:border-emerald-900/60 dark:bg-emerald-950/30">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-100 text-emerald-600 dark:bg-emerald-900/60 dark:text-emerald-300">
              <Icon name="check" size="md" />
            </div>
            <div>
              <p class="text-sm font-semibold text-emerald-700 dark:text-emerald-300">
                {{ result.action === 'activated' ? t('keyExchange.actionActivated') : t('keyExchange.actionQueried') }}
              </p>
              <p class="text-xs text-emerald-600/80 dark:text-emerald-300/80">
                {{ result.activated_at ? formatDateTime(result.activated_at) : '-' }}
              </p>
            </div>
          </div>
        </div>

        <div class="rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
                {{ t('keyExchange.apiKeyLabel') }}
              </p>
              <code class="mt-2 block break-all rounded-2xl bg-gray-100 px-4 py-4 font-mono text-sm text-gray-900 dark:bg-dark-800 dark:text-white">
                {{ result.api_key }}
              </code>
            </div>
              <button
                class="inline-flex items-center justify-center rounded-2xl border border-gray-200 px-4 py-3 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-dark-700 dark:text-dark-100 dark:hover:bg-dark-800"
                @click="copyToClipboard(result.api_key)"
              >
              <Icon name="copy" size="sm" class="mr-2" />
              {{ t('keyExchange.copyKey') }}
            </button>
          </div>

          <div class="mt-4 rounded-2xl bg-gray-50 p-4 dark:bg-dark-950/60">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0 flex-1">
                <p class="text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">
                  {{ t('keyExchange.apiBaseUrl') }}
                </p>
                <code class="mt-2 block break-all font-mono text-sm text-gray-900 dark:text-white">
                  {{ apiBaseUrl }}
                </code>
              </div>
              <button
                class="inline-flex items-center justify-center rounded-2xl border border-gray-200 px-4 py-3 text-sm font-medium text-gray-700 transition hover:bg-white dark:border-dark-700 dark:text-dark-100 dark:hover:bg-dark-900"
                @click="copyToClipboard(apiBaseUrl)"
              >
                <Icon name="copy" size="sm" class="mr-2" />
                {{ t('keyExchange.copyBaseUrl') }}
              </button>
            </div>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <div class="rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <h2 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">
              {{ t('keyExchange.quota') }}
            </h2>
            <div class="mt-4 space-y-3">
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.quota') }}</span>
                <span class="font-medium">{{ quotaLabel(result.quota) }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.quotaUsed') }}</span>
                <span class="font-medium">${{ result.quota_used.toFixed(4) }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.remainingQuota') }}</span>
                <span class="font-medium">{{ remainingQuotaLabel }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.expiresAt') }}</span>
                <span class="font-medium">
                  {{ result.expires_at ? formatDateTime(result.expires_at) : t('keyExchange.noExpiration') }}
                </span>
              </div>
            </div>
          </div>

          <div class="rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <h2 class="text-sm font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">
              Usage
            </h2>
            <div class="mt-4 space-y-3">
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.todayCost') }}</span>
                <span class="font-medium">${{ result.today_actual_cost.toFixed(4) }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.totalCost') }}</span>
                <span class="font-medium">${{ result.total_actual_cost.toFixed(4) }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.totalRequests') }}</span>
                <span class="font-medium">{{ result.total_requests }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.group') }}</span>
                <span class="font-medium">{{ result.group?.name || '-' }}</span>
              </div>
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.activatedAt') }}</span>
                <span class="font-medium">{{ result.activated_at ? formatDateTime(result.activated_at) : '-' }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { keyExchangeAPI } from '@/api'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { APIKeyExchangeResolveResponse } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const code = ref('')
const resolving = ref(false)
const result = ref<APIKeyExchangeResolveResponse | null>(null)
const isDark = ref(document.documentElement.classList.contains('dark'))

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo)
const apiBaseUrl = computed(() => {
  return appStore.cachedPublicSettings?.api_base_url || `${window.location.origin}/v1`
})

const remainingQuotaLabel = computed(() => {
  if (!result.value) return '-'
  if (result.value.quota <= 0) return t('keyExchange.unlimited')
  return `$${Math.max(result.value.quota - result.value.quota_used, 0).toFixed(4)}`
})

function quotaLabel(quota: number): string {
  if (quota <= 0) {
    return t('keyExchange.unlimited')
  }
  return `$${quota.toFixed(4)}`
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

async function handleResolve() {
  const trimmed = code.value.trim().toUpperCase()
  if (!trimmed) {
    appStore.showInfo(t('keyExchange.codeRequired'))
    return
  }

  resolving.value = true
  try {
    result.value = await keyExchangeAPI.resolve(trimmed, Intl.DateTimeFormat().resolvedOptions().timeZone)
    code.value = trimmed
    appStore.showSuccess(
      result.value.action === 'activated'
        ? t('keyExchange.actionActivated')
        : t('keyExchange.actionQueried')
    )
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || error?.message || t('keyExchange.resolveFailed'))
  } finally {
    resolving.value = false
  }
}

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
