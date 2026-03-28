<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
            <input
              v-model="search"
              type="text"
              :placeholder="t('admin.keyExchange.searchPlaceholder')"
              class="input w-full sm:max-w-xs"
              @input="handleSearch"
            />
            <Select
              v-model="filters.status"
              :options="statusOptions"
              class="w-full sm:w-44"
              @change="loadCodes"
            />
          </div>
          <div class="flex flex-wrap items-center justify-end gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="loadCodes">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="showGenerateDialog = true">
              {{ t('admin.keyExchange.generateCodes') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="codes" :loading="loading">
          <template #cell-code="{ value }">
            <div class="flex items-center gap-2">
              <code class="font-mono text-xs text-gray-900 dark:text-white">{{ value }}</code>
              <button
                class="text-gray-400 transition hover:text-gray-600 dark:hover:text-gray-200"
                @click="copyToClipboard(String(value))"
              >
                <Icon name="copy" size="sm" />
              </button>
            </div>
          </template>

          <template #cell-status="{ value }">
            <span
              :class="[
                'badge',
                value === 'unused'
                  ? 'badge-success'
                  : value === 'activated'
                    ? 'badge-primary'
                    : 'badge-danger'
              ]"
            >
              {{ t(`admin.keyExchange.${value}`) }}
            </span>
          </template>

          <template #cell-quota="{ value }">
            <span class="text-sm font-medium">
              {{ Number(value) > 0 ? `$${Number(value).toFixed(2)}` : t('keyExchange.unlimited') }}
            </span>
          </template>

          <template #cell-expires_in_days="{ value }">
            <span class="text-sm">{{ Number(value) > 0 ? value : t('keyExchange.noExpiration') }}</span>
          </template>

          <template #cell-api_key="{ row }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ row.api_key?.key || '-' }}
            </span>
          </template>

          <template #cell-activated_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ value ? formatDateTime(String(value)) : '-' }}
            </span>
          </template>

          <template #cell-group="{ row }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ row.group?.name || '-' }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button
                v-if="row.status === 'unused'"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                @click="confirmDelete(row)"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
              <span v-else class="text-gray-400">-</span>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="t('admin.keyExchange.deleteConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />

    <Teleport to="body">
      <div v-if="showGenerateDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="showGenerateDialog = false"></div>
        <div class="relative z-10 w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.keyExchange.generateTitle') }}
          </h2>

          <form class="mt-5 space-y-4" @submit.prevent="handleGenerate">
            <div>
              <label class="input-label">{{ t('admin.keyExchange.count') }}</label>
              <input v-model.number="form.count" type="number" min="1" max="500" class="input" />
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.keyExchange.countHint') }}</p>
            </div>

            <div>
              <label class="input-label">{{ t('admin.keyExchange.group') }}</label>
              <Select v-model="form.group_id" :options="groupOptions" />
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label class="input-label">{{ t('admin.keyExchange.quota') }}</label>
                <input v-model.number="form.quota" type="number" min="0" step="0.01" class="input" />
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.keyExchange.quotaHint') }}</p>
              </div>
              <div>
                <label class="input-label">{{ t('admin.keyExchange.expiresInDays') }}</label>
                <input v-model.number="form.expires_in_days" type="number" min="0" class="input" />
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.keyExchange.expiresHint') }}</p>
              </div>
            </div>

            <div>
              <label class="input-label">{{ t('admin.keyExchange.batchNo') }}</label>
              <input v-model="form.batch_no" type="text" class="input" />
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.keyExchange.batchNoHint') }}</p>
            </div>

            <div>
              <label class="input-label">{{ t('admin.keyExchange.notes') }}</label>
              <textarea v-model="form.notes" rows="3" class="input min-h-[96px]"></textarea>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.keyExchange.notesHint') }}</p>
            </div>

            <div class="flex justify-end gap-3 pt-2">
              <button type="button" class="btn btn-secondary" @click="showGenerateDialog = false">
                {{ t('common.cancel') }}
              </button>
              <button type="submit" class="btn btn-primary" :disabled="generating">
                {{ generating ? t('common.loading') : t('admin.keyExchange.generateCodes') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showResultDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="showResultDialog = false"></div>
        <div class="relative z-10 w-full max-w-xl rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.keyExchange.generatedSuccess') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ generatedCodes.length }} codes
              </p>
            </div>
            <button class="rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-700" @click="showResultDialog = false">
              <Icon name="x" size="sm" />
            </button>
          </div>

          <textarea
            readonly
            class="mt-4 h-64 w-full rounded-2xl border border-gray-200 bg-gray-50 p-4 font-mono text-sm text-gray-900 dark:border-dark-700 dark:bg-dark-950 dark:text-white"
            :value="generatedCodes.map((item) => item.code).join('\n')"
          ></textarea>

          <div class="mt-4 flex justify-end gap-3">
            <button
              class="btn btn-secondary"
              @click="copyToClipboard(generatedCodes.map((item) => item.code).join('\n'))"
            >
              {{ t('common.copy') }}
            </button>
            <button class="btn btn-primary" @click="showResultDialog = false">
              {{ t('common.confirm') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { keyExchangeAPI, groupsAPI } from '@/api/admin'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { APIKeyExchangeCode, SelectOption, AdminGroup } from '@/types'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const generating = ref(false)
const codes = ref<APIKeyExchangeCode[]>([])
const generatedCodes = ref<APIKeyExchangeCode[]>([])
const groups = ref<AdminGroup[]>([])
const search = ref('')
const searchTimer = ref<number | null>(null)
const filters = ref<{ status: '' | 'unused' | 'activated' | 'disabled' }>({
  status: ''
})
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1
})

const showGenerateDialog = ref(false)
const showResultDialog = ref(false)
const showDeleteDialog = ref(false)
const deletingId = ref<number | null>(null)

const form = ref({
  count: 20,
  group_id: null as number | null,
  quota: 0,
  expires_in_days: 0,
  batch_no: '',
  notes: ''
})

const statusOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('admin.keyExchange.allStatus') },
  { value: 'unused', label: t('admin.keyExchange.unused') },
  { value: 'activated', label: t('admin.keyExchange.activated') },
  { value: 'disabled', label: t('admin.keyExchange.disabled') }
])

const groupOptions = computed<SelectOption[]>(() => [
  { value: null, label: '-' },
  ...groups.value.map((group) => ({
    value: group.id,
    label: group.name
  }))
])

const columns = computed<Column[]>(() => [
  { key: 'code', label: t('admin.keyExchange.code') },
  { key: 'status', label: t('admin.keyExchange.status') },
  { key: 'batch_no', label: t('admin.keyExchange.batchNo') },
  { key: 'quota', label: t('admin.keyExchange.quota') },
  { key: 'expires_in_days', label: t('admin.keyExchange.expiresInDays') },
  { key: 'group', label: t('admin.keyExchange.group') },
  { key: 'api_key', label: t('admin.keyExchange.linkedKey') },
  { key: 'activated_at', label: t('admin.keyExchange.activatedAt') },
  { key: 'actions', label: t('common.actions') }
])

async function loadGroups() {
  try {
    groups.value = await groupsAPI.getAll()
  } catch {
    groups.value = []
  }
}

async function loadCodes() {
  loading.value = true
  try {
    const data = await keyExchangeAPI.list(pagination.value.page, pagination.value.page_size, {
      status: filters.value.status,
      search: search.value.trim() || undefined
    })
    codes.value = data.items
    pagination.value = {
      page: data.page,
      page_size: data.page_size,
      total: data.total,
      pages: data.pages
    }
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || error?.message || t('admin.keyExchange.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  if (searchTimer.value) {
    window.clearTimeout(searchTimer.value)
  }
  searchTimer.value = window.setTimeout(() => {
    pagination.value.page = 1
    loadCodes()
  }, 250)
}

function handlePageChange(page: number) {
  pagination.value.page = page
  loadCodes()
}

function handlePageSizeChange(pageSize: number) {
  pagination.value.page = 1
  pagination.value.page_size = pageSize
  loadCodes()
}

function confirmDelete(row: APIKeyExchangeCode) {
  deletingId.value = row.id
  showDeleteDialog.value = true
}

async function handleDelete() {
  if (!deletingId.value) return
  try {
    await keyExchangeAPI.delete(deletingId.value)
    appStore.showSuccess(t('admin.keyExchange.deletedSuccess'))
    showDeleteDialog.value = false
    deletingId.value = null
    await loadCodes()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || error?.message || t('admin.keyExchange.failedToDelete'))
  }
}

async function handleGenerate() {
  generating.value = true
  try {
    generatedCodes.value = await keyExchangeAPI.generate({
      count: form.value.count,
      group_id: form.value.group_id,
      quota: form.value.quota,
      expires_in_days: form.value.expires_in_days,
      batch_no: form.value.batch_no.trim() || undefined,
      notes: form.value.notes.trim() || undefined
    })
    appStore.showSuccess(t('admin.keyExchange.generatedSuccess'))
    showGenerateDialog.value = false
    showResultDialog.value = true
    await loadCodes()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.detail || error?.message || t('admin.keyExchange.failedToGenerate'))
  } finally {
    generating.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadGroups(), loadCodes()])
})
</script>
