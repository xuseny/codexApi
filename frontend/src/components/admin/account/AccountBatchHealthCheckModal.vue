<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.batchHealthCheck.title')"
    width="wide"
    :close-on-click-outside="false"
    @close="handleClose"
  >
    <div class="space-y-5">
      <div class="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-800 dark:border-blue-900/40 dark:bg-blue-900/20 dark:text-blue-200">
        {{ t('admin.accounts.batchHealthCheck.description', { count: accountIds.length }) }}
      </div>

      <div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
        <div>
          <label class="input-label mb-1.5 block">{{ t('admin.accounts.testModel') }}</label>
          <Select
            v-model="selectedModelId"
            :options="modelOptions"
            :placeholder="t('admin.accounts.batchHealthCheck.modelPlaceholder')"
            searchable
            creatable
            :creatable-prefix="t('admin.accounts.batchHealthCheck.useCustomModel')"
            :disabled="running"
          />
          <p class="input-hint mt-1.5">{{ t('admin.accounts.batchHealthCheck.modelHint') }}</p>
        </div>
        <button
          class="btn btn-primary"
          :disabled="running || !selectedModelId || accountIds.length === 0"
          @click="startHealthCheck"
        >
          {{ running ? t('admin.accounts.batchHealthCheck.running') : t('admin.accounts.batchHealthCheck.start') }}
        </button>
      </div>

      <div v-if="requestError" class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/40 dark:bg-rose-900/20 dark:text-rose-300">
        {{ requestError }}
      </div>

      <div v-if="summary" class="grid gap-3 sm:grid-cols-4">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.batchHealthCheck.total') }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ summary.total }}</div>
        </div>
        <div class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 dark:border-emerald-900/40 dark:bg-emerald-900/20">
          <div class="text-xs text-emerald-700 dark:text-emerald-300">{{ t('admin.accounts.batchHealthCheck.success') }}</div>
          <div class="mt-1 text-xl font-semibold text-emerald-700 dark:text-emerald-200">{{ summary.success }}</div>
        </div>
        <div class="rounded-lg border border-rose-200 bg-rose-50 p-3 dark:border-rose-900/40 dark:bg-rose-900/20">
          <div class="text-xs text-rose-700 dark:text-rose-300">{{ t('admin.accounts.batchHealthCheck.failed') }}</div>
          <div class="mt-1 text-xl font-semibold text-rose-700 dark:text-rose-200">{{ summary.failed }}</div>
        </div>
        <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-900/40 dark:bg-amber-900/20">
          <div class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.accounts.batchHealthCheck.markedError') }}</div>
          <div class="mt-1 text-xl font-semibold text-amber-700 dark:text-amber-200">{{ summary.marked_error }}</div>
        </div>
      </div>

      <div v-if="results.length > 0" class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
        <div class="max-h-80 overflow-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="sticky top-0 bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.batchHealthCheck.account') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.batchHealthCheck.result') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.batchHealthCheck.latency') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.batchHealthCheck.error') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="item in results" :key="item.account_id">
                <td class="whitespace-nowrap px-3 py-2 font-medium text-gray-900 dark:text-white">
                  {{ accountName(item.account_id) }}
                </td>
                <td class="whitespace-nowrap px-3 py-2">
                  <span :class="statusBadgeClass(item)">
                    {{ statusLabel(item) }}
                  </span>
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ typeof item.latency_ms === 'number' && item.latency_ms > 0 ? `${item.latency_ms} ms` : '-' }}
                </td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  <span class="line-clamp-2 break-all" :title="item.error || ''">{{ item.error || '-' }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer>
      <button class="btn btn-secondary" :disabled="running" @click="handleClose">
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type { BatchHealthCheckItem, BatchHealthCheckResult } from '@/api/admin/accounts'
import type { Account, ClaudeModel } from '@/types'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  accounts: Account[]
  accountIds: number[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'completed', result: BatchHealthCheckResult): void
}>()

const selectedModelId = ref('')
const models = ref<ClaudeModel[]>([])
const running = ref(false)
const loadingModels = ref(false)
const summary = ref<BatchHealthCheckResult | null>(null)
const results = ref<BatchHealthCheckItem[]>([])
const requestError = ref('')

const selectedAccounts = computed(() => {
  const ids = new Set(props.accountIds)
  return props.accounts.filter(account => ids.has(account.id))
})

const modelOptions = computed<SelectOption[]>(() => {
  if (loadingModels.value) {
    return [{ value: '', label: t('admin.accounts.batchHealthCheck.loadingModels'), disabled: true }]
  }
  return models.value.map(model => ({
    value: model.id,
    label: model.display_name && model.display_name !== model.id
      ? `${model.display_name} (${model.id})`
      : model.id
  }))
})

watch(
  () => props.show,
  async (visible) => {
    if (!visible) return
    summary.value = null
    results.value = []
    requestError.value = ''
    selectedModelId.value = ''
    await loadModels()
  }
)

const loadModels = async () => {
  const firstAccount = selectedAccounts.value[0]
  if (!firstAccount) return

  loadingModels.value = true
  try {
    models.value = await adminAPI.accounts.getAvailableModels(firstAccount.id)
    selectedModelId.value = pickDefaultModel(firstAccount, models.value)
  } catch (error) {
    console.error('Failed to load account models for batch health check:', error)
    models.value = []
    selectedModelId.value = ''
  } finally {
    loadingModels.value = false
  }
}

const pickDefaultModel = (account: Account, availableModels: ClaudeModel[]) => {
  if (availableModels.length === 0) return ''
  if (account.platform === 'gemini' || account.platform === 'windsurf') {
    const flash = availableModels.find(model => model.id.includes('flash'))
    return flash?.id || availableModels[0].id
  }
  const sonnet = availableModels.find(model => model.id.includes('sonnet'))
  return sonnet?.id || availableModels[0].id
}

const accountName = (accountID: number) => {
  const account = props.accounts.find(item => item.id === accountID)
  return account ? `${account.name} (#${accountID})` : `#${accountID}`
}

const statusLabel = (item: BatchHealthCheckItem) => {
  if (item.success) return t('admin.accounts.batchHealthCheck.statusSuccess')
  if (item.status === 'unauthorized') {
    return item.marked_error
      ? t('admin.accounts.batchHealthCheck.statusUnauthorizedMarked')
      : t('admin.accounts.batchHealthCheck.statusUnauthorized')
  }
  return t('admin.accounts.batchHealthCheck.statusFailed')
}

const statusBadgeClass = (item: BatchHealthCheckItem) => {
  const base = 'inline-flex rounded-full px-2 py-0.5 text-xs font-medium'
  if (item.success) return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300`
  if (item.status === 'unauthorized') return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
  return `${base} bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300`
}

const startHealthCheck = async () => {
  const modelID = String(selectedModelId.value || '').trim()
  if (!modelID || props.accountIds.length === 0) return

  running.value = true
  summary.value = null
  results.value = []
  requestError.value = ''
  try {
    const result = await adminAPI.accounts.batchHealthCheck(props.accountIds, modelID)
    summary.value = result
    results.value = result.results ?? []
    emit('completed', result)
  } catch (error) {
    requestError.value = error instanceof Error ? error.message : t('admin.accounts.batchHealthCheck.requestFailed')
  } finally {
    running.value = false
  }
}

const handleClose = () => {
  if (running.value) return
  emit('close')
}
</script>
