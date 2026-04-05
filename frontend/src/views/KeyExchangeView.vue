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
        <div class="mt-5 flex flex-col items-center justify-center gap-3 sm:flex-row">
          <button
            type="button"
            class="btn btn-secondary w-full sm:w-auto"
            @click="openAfterSalesGroup"
          >
            <Icon name="users" size="sm" class="mr-2" />
            加入售后群
          </button>
          <button
            type="button"
            class="btn btn-primary w-full sm:w-auto"
            @click="openRedeemCodePurchase"
          >
            <Icon name="externalLink" size="sm" class="mr-2" />
            购买兑换码
          </button>
        </div>
      </section>

      <section class="mx-auto mt-8 w-full max-w-3xl rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-8">
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
                class="w-full rounded-2xl border border-gray-200 bg-white px-4 py-3 text-base uppercase tracking-[0.18em] outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-200 dark:border-dark-700 dark:bg-dark-950 dark:focus:ring-primary-900"
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

      <section v-if="result" class="mx-auto mt-6 grid w-full max-w-3xl gap-4">
        <div class="rounded-3xl border border-emerald-200 bg-emerald-50 p-5 dark:border-emerald-900/60 dark:bg-emerald-950/30">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-full bg-emerald-100 text-emerald-600 dark:bg-emerald-900/60 dark:text-emerald-300">
              <Icon name="check" size="md" />
            </div>
            <div>
              <p class="text-sm font-semibold text-emerald-700 dark:text-emerald-300">
                {{ result.action === 'activated' ? t('keyExchange.actionActivated') : t('keyExchange.actionQueried') }}
              </p>
              <p class="text-xs text-emerald-700/80 dark:text-emerald-300/80">
                {{ result.group?.name || '-' }} · {{ result.activated_at ? formatDateTime(result.activated_at) : '-' }}
              </p>
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
              {{ t('keyExchange.usageTitle') }}
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
                <span class="text-gray-500 dark:text-dark-400">{{ t('keyExchange.statusLabel') }}</span>
                <span class="font-medium">{{ t(`keyExchange.status.${result.api_key_status}`) }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="rounded-3xl border border-amber-200 bg-white p-5 shadow-sm dark:border-amber-900/60 dark:bg-dark-900 sm:p-6">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('keyExchange.quotaRechargeTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('keyExchange.quotaRechargeDescription') }}
              </p>
            </div>

            <div
              class="rounded-2xl px-3 py-2 text-xs"
              :class="canRedeemQuota
                ? 'border border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300'
                : 'border border-gray-200 bg-gray-50 text-gray-500 dark:border-dark-700 dark:bg-dark-950/60 dark:text-dark-300'"
            >
              {{ canRedeemQuota ? t('keyExchange.quotaRechargeAvailable') : quotaRechargeUnavailableReason }}
            </div>
          </div>

          <form class="mt-5 space-y-4" @submit.prevent="handleRedeemQuota">
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">
                {{ t('keyExchange.redeemCodeLabel') }}
              </label>
              <div class="flex flex-col gap-3 sm:flex-row">
                <input
                  v-model="quotaRedeemCode"
                  type="text"
                  autocapitalize="characters"
                  spellcheck="false"
                  :placeholder="t('keyExchange.redeemCodePlaceholder')"
                  :disabled="redeemingQuota || !canRedeemQuota"
                  class="w-full rounded-2xl border border-gray-200 bg-white px-4 py-3 text-base outline-none transition focus:border-primary-500 focus:ring-2 focus:ring-primary-200 disabled:cursor-not-allowed disabled:opacity-60 dark:border-dark-700 dark:bg-dark-950 dark:focus:ring-primary-900"
                />
                <button
                  type="submit"
                  :disabled="redeemingQuota || !canRedeemQuota"
                  class="inline-flex min-w-[180px] items-center justify-center rounded-2xl bg-amber-500 px-5 py-3 text-sm font-medium text-white transition hover:bg-amber-600 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <Icon v-if="!redeemingQuota" name="gift" size="sm" class="mr-2" />
                  <svg v-else class="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
                    <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" opacity="0.25" />
                    <path d="M12 2a10 10 0 0 1 10 10" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
                  </svg>
                  {{ redeemingQuota ? t('keyExchange.quotaRecharging') : t('keyExchange.quotaRechargeButton') }}
                </button>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('keyExchange.quotaRechargeHint') }}
              </p>
            </div>
          </form>
        </div>

        <div class="rounded-3xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('keyExchange.configTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('keyExchange.configDescription') }}
              </p>
            </div>

            <div class="rounded-2xl border border-dashed border-gray-300 px-3 py-2 text-xs text-gray-500 dark:border-dark-600 dark:text-dark-300">
              {{ fileSystemAccessSupported ? t('keyExchange.saveSupported') : t('keyExchange.saveNotSupported') }}
            </div>
          </div>

          <div v-if="configPresets.length" class="mt-5 flex flex-wrap gap-2">
            <button
              v-for="preset in configPresets"
              :key="preset.id"
              type="button"
              class="rounded-2xl border px-4 py-2 text-sm font-medium transition"
              :class="selectedPresetId === preset.id
                ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200 dark:hover:bg-dark-800'"
              @click="selectedPresetId = preset.id"
            >
              {{ preset.label }}
            </button>
          </div>

          <div v-if="selectedPreset" class="mt-5">
            <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-950/60">
              <p class="text-sm text-gray-700 dark:text-dark-200">{{ selectedPreset.description }}</p>
            </div>

            <div class="mt-4 space-y-4">
              <div
                v-for="file in selectedPreset.files"
                :key="`${selectedPreset.id}-${file.fileName}`"
                class="rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-950/40"
              >
                <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                        {{ file.fileName }}
                      </span>
                      <span class="text-xs text-gray-500 dark:text-dark-400">
                        {{ file.suggestedPath }}
                      </span>
                    </div>
                    <p v-if="file.hint" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                      {{ file.hint }}
                    </p>
                  </div>

                  <div class="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      class="btn btn-secondary"
                      @click="downloadConfigFile(file)"
                    >
                      <Icon name="download" size="sm" class="mr-2" />
                      {{ t('keyExchange.downloadConfig') }}
                    </button>
                    <button
                      v-if="fileSystemAccessSupported"
                      type="button"
                      class="btn btn-primary"
                      @click="saveConfigFile(file)"
                    >
                      <Icon name="edit" size="sm" class="mr-2" />
                      {{ t('keyExchange.saveToLocal') }}
                    </button>
                  </div>
                </div>

                <details class="mt-4 rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-900/70">
                  <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-dark-200">
                    {{ t('keyExchange.previewConfig') }}
                  </summary>
                  <pre class="mt-3 overflow-x-auto whitespace-pre-wrap break-all rounded-xl bg-gray-900 p-4 text-xs text-gray-100">{{ file.content }}</pre>
                </details>
              </div>

              <div
                v-if="showCcsImportCard"
                class="rounded-2xl border border-blue-200 bg-blue-50/80 p-4 dark:border-blue-900/60 dark:bg-blue-950/20"
              >
                <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                      <span class="rounded-full bg-blue-100 px-2.5 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                        CC-Switch
                      </span>
                      <span class="text-sm font-semibold text-gray-900 dark:text-white">
                        {{ t('keys.importToCcSwitch') }}
                      </span>
                    </div>
                    <div class="mt-3 space-y-1 text-xs text-gray-500 dark:text-dark-400">
                      <p>{{ t('common.name') }}: {{ siteName }}</p>
                      <p>{{ t('keyExchange.apiBaseUrl') }}: {{ ccsImportEndpoint }}</p>
                    </div>
                  </div>

                  <button
                    type="button"
                    class="btn btn-primary"
                    @click="handleImportToCcs"
                  >
                    <Icon name="upload" size="sm" class="mr-2" />
                    {{ t('keys.importToCcSwitch') }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div
            v-else
            class="mt-5 rounded-2xl border border-dashed border-gray-300 bg-gray-50 p-4 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-950/40 dark:text-dark-300"
          >
            {{ t('keyExchange.configUnavailable') }}
          </div>
        </div>

        <details class="rounded-3xl border border-amber-200 bg-amber-50/80 p-5 shadow-sm dark:border-amber-900/60 dark:bg-amber-950/20">
          <summary class="cursor-pointer text-sm font-semibold text-amber-700 dark:text-amber-300">
            {{ t('keyExchange.advancedToggle') }}
          </summary>
          <div class="mt-4 space-y-4">
            <div class="rounded-2xl border border-amber-200 bg-white p-4 dark:border-amber-900/60 dark:bg-dark-900">
              <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
                    {{ t('keyExchange.apiKeyLabel') }}
                  </p>
                  <code class="mt-2 block break-all rounded-2xl bg-gray-100 px-4 py-4 font-mono text-sm text-gray-900 dark:bg-dark-800 dark:text-white">
                    {{ result.api_key }}
                  </code>
                </div>
                <button class="btn btn-secondary" @click="copyToClipboard(result.api_key)">
                  <Icon name="copy" size="sm" class="mr-2" />
                  {{ t('keyExchange.copyKey') }}
                </button>
              </div>
            </div>

            <div class="rounded-2xl border border-amber-200 bg-white p-4 dark:border-amber-900/60 dark:bg-dark-900">
              <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
                    {{ t('keyExchange.apiBaseUrl') }}
                  </p>
                  <code class="mt-2 block break-all rounded-2xl bg-gray-100 px-4 py-4 font-mono text-sm text-gray-900 dark:bg-dark-800 dark:text-white">
                    {{ apiBaseUrl }}
                  </code>
                </div>
                <button class="btn btn-secondary" @click="copyToClipboard(apiBaseUrl)">
                  <Icon name="copy" size="sm" class="mr-2" />
                  {{ t('keyExchange.copyBaseUrl') }}
                </button>
              </div>
            </div>
          </div>
        </details>
      </section>
    </main>

    <BaseDialog
      :show="showCcsClientSelect"
      :title="t('keys.ccsClientSelect.title')"
      width="narrow"
      @close="closeCcsClientSelect"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-400">
          {{ t('keys.ccsClientSelect.description') }}
        </p>
        <div class="grid grid-cols-2 gap-3">
          <button
            @click="handleCcsClientSelect('claude')"
            class="flex flex-col items-center gap-2 rounded-xl border-2 border-gray-200 p-4 transition-all hover:border-primary-500 hover:bg-primary-50 dark:border-dark-600 dark:hover:border-primary-500 dark:hover:bg-primary-900/20"
          >
            <Icon name="terminal" size="xl" class="text-gray-600 dark:text-gray-400" />
            <span class="font-medium text-gray-900 dark:text-white">
              {{ t('keys.ccsClientSelect.claudeCode') }}
            </span>
            <span class="text-center text-xs text-gray-500 dark:text-gray-400">
              {{ t('keys.ccsClientSelect.claudeCodeDesc') }}
            </span>
          </button>
          <button
            @click="handleCcsClientSelect('gemini')"
            class="flex flex-col items-center gap-2 rounded-xl border-2 border-gray-200 p-4 transition-all hover:border-primary-500 hover:bg-primary-50 dark:border-dark-600 dark:hover:border-primary-500 dark:hover:bg-primary-900/20"
          >
            <Icon name="sparkles" size="xl" class="text-gray-600 dark:text-gray-400" />
            <span class="font-medium text-gray-900 dark:text-white">
              {{ t('keys.ccsClientSelect.geminiCli') }}
            </span>
            <span class="text-center text-xs text-gray-500 dark:text-gray-400">
              {{ t('keys.ccsClientSelect.geminiCliDesc') }}
            </span>
          </button>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeCcsClientSelect" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { keyExchangeAPI } from '@/api'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { buildKeyExchangeConfigPresets, type KeyExchangeConfigFile } from '@/utils/keyExchangeConfig'
import type { APIKeyExchangeResolveResponse } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const AFTER_SALES_QQ_GROUP_NUMBER = '427293497'
const AFTER_SALES_QQ_GROUP_DEEPLINK = `mqqapi://card/show_pslcard?src_type=internal&version=1&card_type=group&uin=${AFTER_SALES_QQ_GROUP_NUMBER}&source=qrcode`
const REDEEM_CODE_PURCHASE_URL = 'https://pay.ldxp.cn/shop/CZYP4OG5'

const code = ref('')
const resolving = ref(false)
const result = ref<APIKeyExchangeResolveResponse | null>(null)
const quotaRedeemCode = ref('')
const redeemingQuota = ref(false)
const isDark = ref(document.documentElement.classList.contains('dark'))
const selectedPresetId = ref<string>('')
const showCcsClientSelect = ref(false)

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo)
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || `${window.location.origin}/v1`)
const ccsImportBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || window.location.origin)

const fileSystemAccessSupported = computed(() => {
  return typeof window !== 'undefined'
    && window.isSecureContext
    && typeof (window as any).showSaveFilePicker === 'function'
})

const configPresets = computed(() => {
  if (!result.value?.group?.platform) {
    return []
  }
  return buildKeyExchangeConfigPresets({
    platform: result.value.group.platform,
    baseUrl: apiBaseUrl.value,
    apiKey: result.value.api_key
  })
})

const selectedPreset = computed(() => {
  return configPresets.value.find((item) => item.id === selectedPresetId.value) || configPresets.value[0] || null
})

const showCcsImportCard = computed(() => {
  return Boolean(
    result.value?.api_key
    && result.value?.group?.platform
    && selectedPreset.value?.id === 'opencode'
    && !appStore.cachedPublicSettings?.hide_ccs_import_button
  )
})

const ccsImportEndpoint = computed(() => {
  const platform = result.value?.group?.platform
  if (platform === 'antigravity') {
    return `${ccsImportBaseUrl.value}/antigravity`
  }
  return ccsImportBaseUrl.value
})

const remainingQuotaLabel = computed(() => {
  if (!result.value) return '-'
  if (result.value.quota <= 0) return t('keyExchange.unlimited')
  return `$${Math.max(result.value.quota - result.value.quota_used, 0).toFixed(4)}`
})

const canRedeemQuota = computed(() => {
  return Boolean(
    result.value
    && result.value.quota > 0
    && (result.value.api_key_status === 'active' || result.value.api_key_status === 'quota_exhausted')
  )
})

const quotaRechargeUnavailableReason = computed(() => {
  if (!result.value) return t('keyExchange.codeRequired')
  if (result.value.quota <= 0) return t('keyExchange.quotaRechargeUnlimited')
  if (result.value.api_key_status === 'expired') return t('keyExchange.quotaRechargeExpired')
  if (result.value.api_key_status === 'disabled') return t('keyExchange.quotaRechargeDisabled')
  return t('keyExchange.quotaRechargeUnavailable')
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

function openAfterSalesGroup() {
  try {
    window.open(AFTER_SALES_QQ_GROUP_DEEPLINK, '_self')
    window.setTimeout(() => {
      if (document.hasFocus()) {
        appStore.showInfo(`如果没有自动拉起 QQ，请手动搜索群 ${AFTER_SALES_QQ_GROUP_NUMBER}`)
      }
    }, 100)
  } catch (error) {
    appStore.showInfo(`请手动搜索 QQ 群 ${AFTER_SALES_QQ_GROUP_NUMBER}`)
  }
}

function openRedeemCodePurchase() {
  const opened = window.open(REDEEM_CODE_PURCHASE_URL, '_blank', 'noopener,noreferrer')
  if (!opened) {
    window.location.href = REDEEM_CODE_PURCHASE_URL
  }
}

function executeCcsImport(clientType: 'claude' | 'gemini') {
  if (!result.value?.api_key) {
    return
  }

  const platform = result.value.group?.platform || 'anthropic'
  const baseUrl = ccsImportBaseUrl.value

  let app: string
  let endpoint: string

  if (platform === 'antigravity') {
    app = clientType === 'gemini' ? 'gemini' : 'claude'
    endpoint = `${baseUrl}/antigravity`
  } else {
    switch (platform) {
      case 'openai':
        app = 'codex'
        endpoint = baseUrl
        break
      case 'gemini':
        app = 'gemini'
        endpoint = baseUrl
        break
      default:
        app = 'claude'
        endpoint = baseUrl
    }
  }

  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`

  const providerName = siteName.value.trim() || 'sub2api'
  const params = new URLSearchParams({
    resource: 'provider',
    app,
    name: providerName,
    homepage: baseUrl,
    endpoint,
    apiKey: result.value.api_key,
    configFormat: 'json',
    usageEnabled: 'true',
    usageScript: btoa(usageScript),
    usageAutoInterval: '30'
  })

  try {
    window.open(`ccswitch://v1/import?${params.toString()}`, '_self')
    setTimeout(() => {
      if (document.hasFocus()) {
        appStore.showError(t('keys.ccSwitchNotInstalled'))
      }
    }, 100)
  } catch (error) {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  }
}

function handleImportToCcs() {
  const platform = result.value?.group?.platform || 'anthropic'
  if (platform === 'antigravity') {
    showCcsClientSelect.value = true
    return
  }
  executeCcsImport(platform === 'gemini' ? 'gemini' : 'claude')
}

function handleCcsClientSelect(clientType: 'claude' | 'gemini') {
  executeCcsImport(clientType)
  showCcsClientSelect.value = false
}

function closeCcsClientSelect() {
  showCcsClientSelect.value = false
}

function downloadConfigFile(file: KeyExchangeConfigFile) {
  const blob = new Blob([file.content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = file.fileName
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

async function saveConfigFile(file: KeyExchangeConfigFile) {
  try {
    const picker = (window as any).showSaveFilePicker
    if (typeof picker !== 'function') {
      appStore.showWarning(t('keyExchange.saveNotSupported'))
      return
    }

    const extension = file.fileName.includes('.') ? '.' + file.fileName.split('.').pop() : '.txt'
    const handle = await picker({
      suggestedName: file.fileName,
      types: [
        {
          description: 'Config File',
          accept: {
            'text/plain': [extension]
          }
        }
      ]
    })

    const writable = await handle.createWritable()
    await writable.write(file.content)
    await writable.close()
    appStore.showSuccess(t('keyExchange.savedSuccess'))
  } catch (error: any) {
    if (error?.name === 'AbortError') {
      return
    }
    appStore.showError(error?.message || t('keyExchange.savedFailed'))
  }
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
    quotaRedeemCode.value = ''
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

async function handleRedeemQuota() {
  if (!result.value) {
    appStore.showInfo(t('keyExchange.codeRequired'))
    return
  }

  const exchangeCode = (result.value.code || code.value).trim().toUpperCase()
  const redeemCode = quotaRedeemCode.value.trim().toUpperCase()
  if (!exchangeCode) {
    appStore.showInfo(t('keyExchange.codeRequired'))
    return
  }
  if (!redeemCode) {
    appStore.showInfo(t('keyExchange.redeemCodeRequired'))
    return
  }
  if (!canRedeemQuota.value) {
    appStore.showWarning(quotaRechargeUnavailableReason.value)
    return
  }

  redeemingQuota.value = true
  try {
    const response = await keyExchangeAPI.redeemQuota(
      exchangeCode,
      redeemCode,
      Intl.DateTimeFormat().resolvedOptions().timeZone
    )
    if (response.result) {
      result.value = response.result
    }
    quotaRedeemCode.value = ''
    appStore.showSuccess(t('keyExchange.quotaRechargeSuccess', { amount: response.amount.toFixed(2) }))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || error?.message || t('keyExchange.quotaRechargeFailed'))
  } finally {
    redeemingQuota.value = false
  }
}

watch(configPresets, (value) => {
  selectedPresetId.value = value[0]?.id || ''
}, { immediate: true })

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
