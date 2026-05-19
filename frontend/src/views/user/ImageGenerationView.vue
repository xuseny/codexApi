<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/85 backdrop-blur dark:border-dark-800 dark:bg-dark-950/85">
      <div class="mx-auto flex w-full max-w-[1600px] items-center justify-between gap-4 px-4 py-3 sm:px-6">
        <router-link to="/key-exchange" class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-sm">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div>
            <div class="text-lg font-semibold leading-tight">{{ siteName }}</div>
            <div class="text-xs text-gray-500 dark:text-dark-400">AI 画图</div>
          </div>
        </router-link>

        <div class="flex items-center gap-2">
          <LocaleSwitcher />
        </div>
      </div>
    </header>

    <main class="mx-auto grid w-full max-w-[1600px] gap-3 px-3 py-3 lg:grid-cols-[300px_minmax(0,1fr)] lg:items-start lg:px-4">
      <aside class="card flex min-h-0 flex-col overflow-hidden lg:sticky lg:top-3 lg:h-[calc(100dvh-5.5rem)] lg:self-start">
        <div class="border-b border-gray-100 px-4 py-4 dark:border-dark-700">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h1 class="text-lg font-semibold">画图对话</h1>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ conversations.length }} 个会话</p>
            </div>
            <div class="flex items-center gap-2">
              <button class="btn btn-primary btn-sm" type="button" @click="createDraftConversation">
                <Icon name="plus" size="sm" />
                新会话
              </button>
              <button
                class="btn btn-secondary btn-sm px-2"
                type="button"
                :disabled="conversations.length === 0"
                title="清空历史记录"
                @click="openClearHistory"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>
          <div class="mt-3 flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
            <span class="badge badge-gray">本地保存</span>
            <span class="badge badge-gray">生成 / 编辑</span>
          </div>
        </div>

        <div class="min-h-0 px-2 py-2 lg:flex-1 lg:overflow-y-auto">
          <button
            type="button"
            class="mb-2 flex w-full items-center justify-between rounded-xl border px-3 py-3 text-left transition"
            :class="!selectedConversationId ? 'border-primary-200 bg-primary-50 dark:border-primary-900/40 dark:bg-primary-900/20' : 'border-transparent hover:bg-gray-100 dark:hover:bg-dark-800'"
            @click="createDraftConversation"
          >
            <div class="min-w-0">
              <div class="truncate text-sm font-medium">新建草稿</div>
              <div class="truncate text-xs text-gray-500 dark:text-dark-400">输入提示词开始画图</div>
            </div>
            <Icon name="plus" size="sm" class="text-gray-400" />
          </button>

          <button
            v-for="conversation in conversations"
            :key="conversation.id"
            type="button"
            class="group mb-1 flex w-full items-start justify-between gap-3 rounded-xl border px-3 py-3 text-left transition"
            :class="conversation.id === selectedConversationId ? 'border-primary-200 bg-primary-50 dark:border-primary-900/40 dark:bg-primary-900/20' : 'border-transparent hover:bg-gray-100 dark:hover:bg-dark-800'"
            @click="selectConversation(conversation.id)"
          >
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium">{{ conversation.title }}</div>
              <div class="mt-1 flex items-center gap-2 text-[11px] text-gray-500 dark:text-dark-400">
                <span>{{ conversation.turns.length }} 轮</span>
                <span v-if="conversationHasPending(conversation)" class="badge badge-warning">生成中</span>
                <span v-else-if="conversationHasError(conversation)" class="badge badge-danger">有失败</span>
                <span v-else class="badge badge-gray">{{ formatTime(conversation.updatedAt) }}</span>
              </div>
            </div>

            <div class="flex items-center gap-1 opacity-0 transition group-hover:opacity-100">
              <button
                type="button"
                class="rounded-lg p-1 text-gray-400 hover:bg-gray-200 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                @click.stop="renameConversation(conversation)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1 text-gray-400 hover:bg-gray-200 hover:text-red-600 dark:hover:bg-dark-700"
                @click.stop="openDeleteConversation(conversation.id)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </button>
        </div>
      </aside>

      <section class="card flex min-h-0 flex-col overflow-hidden lg:h-[calc(100dvh-5.5rem)]">
        <div class="border-b border-gray-100 px-4 py-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <h2 class="truncate text-xl font-semibold">
                {{ selectedConversation?.title || '新建草稿' }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ selectedConversation ? `共 ${selectedConversation.turns.length} 轮对话` : '先写提示词，再发一轮画图' }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <span class="badge badge-gray">模型 {{ form.model }}</span>
              <span class="badge badge-gray">{{ selectedAspectLabel }}</span>
              <span class="badge badge-gray">{{ selectedResolutionLabel }}</span>
              <span class="badge badge-gray">张数 {{ form.count }}</span>
            </div>
          </div>
        </div>

        <div ref="scrollRef" class="min-h-0 px-3 py-4 sm:px-4 lg:flex-1 lg:overflow-y-auto">
          <div v-if="!selectedConversation || selectedConversation.turns.length === 0" class="flex items-center justify-center py-8 text-center lg:min-h-full lg:py-0">
            <div class="max-w-md text-center">
              <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400">
                <Icon name="sparkles" size="lg" />
              </div>
              <h3 class="mt-4 text-lg font-semibold">开始一段画图对话</h3>
              <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400">
                输入第一句描述后，会生成一个会话。后续可以继续补充提示词，或把结果图带回去继续编辑。
              </p>
            </div>
          </div>

          <div v-else class="space-y-5">
            <article
              v-for="turn in selectedConversation.turns"
              :key="turn.id"
              class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900/40"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                  <span class="badge badge-gray">{{ turn.mode === 'edit' ? '编辑' : '生成' }}</span>
                  <span class="badge badge-gray">{{ imageSizeLabel(turn.size) }}</span>
                  <span class="badge badge-gray">{{ turn.count }} 张</span>
                  <span :class="turnStatusClass(turn.status)" class="badge">{{ turnStatusLabel(turn.status) }}</span>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <button class="btn btn-secondary btn-sm" type="button" @click="reuseTurn(turn)">
                    <Icon name="refresh" size="sm" />
                    复用配置
                  </button>
                  <button class="btn btn-secondary btn-sm" type="button" @click="regenerateTurn(turn)">
                    <Icon name="play" size="sm" />
                    重来一轮
                  </button>
                </div>
              </div>

              <div v-if="!turn.promptDeleted" class="mt-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800/70">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="text-xs font-semibold uppercase text-gray-400">提示词</div>
                    <p class="mt-2 whitespace-pre-wrap text-sm leading-6 text-gray-700 dark:text-dark-200">{{ turn.prompt }}</p>
                  </div>
                  <div class="flex shrink-0 items-center gap-1">
                    <button class="rounded-lg p-2 text-gray-400 hover:bg-white hover:text-red-600 dark:hover:bg-dark-700" type="button" @click="openDeletePrompt(turn.id)">
                      <Icon name="trash" size="sm" />
                    </button>
                    <button class="rounded-lg p-2 text-gray-400 hover:bg-white hover:text-red-600 dark:hover:bg-dark-700" type="button" @click="openDeleteResults(turn.id)">
                      <Icon name="xCircle" size="sm" />
                    </button>
                  </div>
                </div>

              </div>

              <div v-if="!turn.resultsDeleted && turn.referenceImages.length" class="mt-4 flex flex-col items-end">
                <div class="text-xs font-semibold uppercase text-gray-400">本轮参考图</div>
                <div class="mt-2 flex flex-wrap justify-end gap-2">
                  <div
                    v-for="(refImage, index) in turn.referenceImages"
                    :key="`${turn.id}-ref-${index}`"
                    class="flex flex-col items-end gap-2"
                  >
                    <button
                      type="button"
                      class="group relative overflow-hidden rounded-xl border border-gray-200 bg-white"
                      @click="openReferenceLightbox(turn, index)"
                    >
                      <img :src="refImage.dataUrl" :alt="refImage.name || `参考图 ${index + 1}`" class="h-20 w-20 object-cover transition group-hover:scale-[1.02]" />
                      <span class="absolute inset-x-1 bottom-1 rounded-lg bg-white/90 px-2 py-1 text-center text-[11px] font-medium text-gray-700 opacity-0 shadow transition group-hover:opacity-100 dark:bg-dark-900/90 dark:text-dark-100">
                        预览
                      </span>
                    </button>
                    <button class="btn btn-secondary btn-sm" type="button" @click="addReferenceImageToComposer(refImage)">
                      加入编辑
                    </button>
                  </div>
                </div>
              </div>

              <div v-if="!turn.resultsDeleted" class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <div
                  v-for="image in turn.images"
                  :key="image.id"
                  class="overflow-hidden rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"
                >
                  <div v-if="image.status === 'generating'" class="flex aspect-square items-center justify-center text-gray-400">
                    <div class="text-center">
                      <div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500"></div>
                      <div class="mt-3 text-sm">生成中</div>
                    </div>
                  </div>
                  <div v-else-if="image.status === 'error'" class="flex aspect-square items-center justify-center p-4 text-center text-sm text-red-500">
                    <div>
                      <div>{{ image.error || '生成失败' }}</div>
                      <button class="btn btn-secondary btn-sm mt-3" type="button" @click="retryImage(turn.id, image.id)">
                        <Icon name="refresh" size="sm" />
                        重试
                      </button>
                    </div>
                  </div>
                  <button v-else type="button" class="block w-full cursor-zoom-in" @click="openTurnLightbox(turn, image)">
                    <img :src="imageSrc(image)" alt="生成结果" class="aspect-square w-full object-cover" @load="handleImageLoad(image.id, $event)" />
                  </button>
                  <div v-if="image.status === 'success'" class="flex items-center justify-between gap-2 px-3 py-2 text-xs text-gray-500 dark:text-dark-400">
                    <span class="truncate">结果{{ imageMeta(image) ? ` · ${imageMeta(image)}` : '' }}</span>
                    <div class="flex items-center gap-1">
                      <button class="rounded-md px-2 py-1 hover:bg-gray-100 dark:hover:bg-dark-700" type="button" @click="continueEdit(image)">
                        继续编辑
                      </button>
                      <button class="rounded-md px-2 py-1 hover:bg-gray-100 dark:hover:bg-dark-700" type="button" @click="downloadImage(image)">
                        下载
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </article>
          </div>
        </div>

        <div class="border-t border-gray-100 px-3 py-3 dark:border-dark-700 sm:px-4">
          <div v-if="composerReferences.length" class="mb-3 flex flex-wrap gap-2">
            <div
              v-for="(image, index) in composerReferences"
              :key="`${image.name}-${index}`"
              class="relative overflow-hidden rounded-xl border border-gray-200 bg-white"
            >
              <button type="button" class="block" @click="openComposerLightbox(index)">
                <img :src="image.dataUrl" class="h-16 w-16 object-cover" :alt="image.name || `参考图 ${index + 1}`" />
              </button>
              <button class="absolute right-1 top-1 rounded-full bg-white/90 p-1 text-gray-500 shadow" type="button" @click="removeComposerReference(index)">
                <Icon name="x" size="xs" />
              </button>
            </div>
          </div>

          <div class="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900/50">
            <div class="grid gap-3 lg:grid-cols-[1fr_auto]">
              <div class="space-y-2">
                <div class="flex flex-col gap-2 lg:hidden">
                  <label
                    v-if="apiKeyOptions.length > 0 || apiKeysLoading"
                    class="flex min-w-0 items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"
                  >
                    <Icon name="key" size="sm" class="shrink-0 text-gray-400" />
                    <select
                      v-model="form.selectedApiKeyId"
                      class="min-w-0 flex-1 bg-transparent outline-none"
                      :disabled="apiKeysLoading"
                    >
                      <option :value="MANUAL_API_KEY_ID">{{ apiKeysLoading ? '正在读取我的 Key...' : '手动输入 API Key' }}</option>
                      <option v-for="option in apiKeyOptions" :key="option.id" :value="option.id">
                        {{ option.label }}
                      </option>
                    </select>
                  </label>
                  <label
                    v-if="form.selectedApiKeyId === MANUAL_API_KEY_ID || apiKeyOptions.length === 0"
                    class="flex min-w-0 items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"
                  >
                    <Icon name="key" size="sm" class="shrink-0 text-gray-400" />
                    <input
                      v-model="form.apiKey"
                      type="password"
                      autocomplete="off"
                      class="min-w-0 flex-1 bg-transparent outline-none"
                      placeholder="API Key（自动保存）"
                    />
                  </label>
                </div>
                <textarea
                  ref="textareaRef"
                  v-model="form.prompt"
                  class="input min-h-[88px] resize-none border-0 bg-transparent px-1 py-2 shadow-none focus-visible:ring-0 sm:min-h-[110px]"
                  :placeholder="composerReferences.length ? '描述你希望如何修改参考图。Enter 发送，Shift+Enter 换行。' : '写下你要画的内容，也可直接粘贴图片。Enter 发送，Shift+Enter 换行。'"
                  @keydown.enter.exact.prevent="submitTurn"
                  @paste="handleComposerPaste"
                ></textarea>
              </div>

              <div class="flex flex-row flex-wrap items-start gap-2 lg:w-[330px] lg:flex-col">
                <div class="flex w-full flex-wrap gap-2">
                  <div class="hidden min-w-0 flex-col gap-2 lg:flex lg:flex-none">
                    <label
                      v-if="apiKeyOptions.length > 0 || apiKeysLoading"
                      class="flex min-w-0 items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"
                    >
                      <Icon name="key" size="sm" class="shrink-0 text-gray-400" />
                      <select
                        v-model="form.selectedApiKeyId"
                        class="min-w-0 flex-1 bg-transparent outline-none"
                        :disabled="apiKeysLoading"
                      >
                        <option :value="MANUAL_API_KEY_ID">{{ apiKeysLoading ? '正在读取我的 Key...' : '手动输入 API Key' }}</option>
                        <option v-for="option in apiKeyOptions" :key="option.id" :value="option.id">
                          {{ option.label }}
                        </option>
                      </select>
                    </label>
                    <label
                      v-if="form.selectedApiKeyId === MANUAL_API_KEY_ID || apiKeyOptions.length === 0"
                      class="flex min-w-0 items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"
                    >
                      <Icon name="key" size="sm" class="shrink-0 text-gray-400" />
                      <input
                        v-model="form.apiKey"
                        type="password"
                        autocomplete="off"
                        class="min-w-0 flex-1 bg-transparent outline-none"
                        placeholder="API Key（自动保存）"
                      />
                    </label>
                  </div>
                  <button class="btn btn-secondary btn-sm" type="button" @click="pickReferenceImages">
                    <Icon name="upload" size="sm" />
                    参考图
                  </button>
                  <button class="btn btn-secondary btn-sm" type="button" @click="clearComposer">
                    <Icon name="trash" size="sm" />
                    清空
                  </button>
                  <button class="btn btn-secondary btn-sm" type="button" @click="toggleMode">
                    <Icon name="swap" size="sm" />
                    {{ composerReferences.length ? '编辑模式' : '生成模式' }}
                  </button>
                </div>

                <div class="flex w-full flex-wrap gap-2">
                  <label class="flex items-center gap-2 rounded-xl border border-gray-200 px-3 py-2 text-sm dark:border-dark-700">
                    <span class="text-gray-500 dark:text-dark-400">张数</span>
                    <input v-model="form.count" type="number" min="1" max="8" class="w-16 bg-transparent text-right outline-none" />
                  </label>

                  <select v-model="form.aspectRatio" class="input w-auto min-w-[138px]">
                    <option v-for="option in aspectRatioOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>

                  <select v-model="form.resolutionTier" class="input w-auto min-w-[96px]">
                    <option v-for="option in availableResolutionOptions" :key="option.value" :value="option.value">
                      {{ option.label }}
                    </option>
                  </select>

                  <select v-model="form.model" class="input w-auto min-w-[150px]">
                    <option v-for="model in modelOptions" :key="model" :value="model">{{ model }}</option>
                  </select>
                </div>

                <div class="flex w-full items-center justify-between gap-3">
                  <div class="text-xs text-gray-500 dark:text-dark-400">
                    <span class="badge badge-gray">本地草稿</span>
                    <span v-if="pendingCount > 0" class="ml-2 badge badge-warning">{{ pendingCount }} 轮进行中</span>
                  </div>
                  <button class="btn btn-primary" type="button" :disabled="!form.prompt.trim()" @click="submitTurn">
                    <Icon v-if="pendingCount === 0" name="sparkles" size="sm" />
                    <span v-else class="spinner h-4 w-4 border-white/40 border-t-white"></span>
                    发送
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <input ref="fileInputRef" type="file" accept="image/*" multiple class="hidden" @change="handleReferenceFiles" />

    <BaseDialog :show="lightboxOpen" title="图片预览" width="full" close-on-click-outside @close="closeLightbox">
      <div v-if="activeLightboxImage" class="space-y-4">
        <div class="flex max-h-[72vh] items-center justify-center overflow-hidden rounded-2xl bg-gray-100 dark:bg-dark-900">
          <img :src="activeLightboxImage.src" :alt="activeLightboxImage.name || '图片预览'" class="max-h-[72vh] max-w-full object-contain" />
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500 dark:text-dark-400">
          <div class="min-w-0 truncate">
            {{ activeLightboxImage.name || `图片 ${lightboxIndex + 1}` }}
            <span v-if="activeLightboxImage.meta"> · {{ activeLightboxImage.meta }}</span>
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary btn-sm" type="button" :disabled="lightboxImages.length <= 1" @click="previousLightboxImage">
              <Icon name="chevronLeft" size="sm" />
            </button>
            <span>{{ lightboxIndex + 1 }} / {{ lightboxImages.length }}</span>
            <button class="btn btn-secondary btn-sm" type="button" :disabled="lightboxImages.length <= 1" @click="nextLightboxImage">
              <Icon name="chevronRight" size="sm" />
            </button>
            <button v-if="activeLightboxImage.reference" class="btn btn-primary btn-sm" type="button" @click="addReferenceImageToComposer(activeLightboxImage.reference)">
              加入编辑
            </button>
          </div>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="renameDialogOpen" title="重命名会话" width="narrow" @close="closeRenameDialog">
      <div class="space-y-3">
        <input v-model="renameValue" class="input" type="text" maxlength="80" />
      </div>
      <template #footer>
        <button class="btn btn-secondary" type="button" @click="closeRenameDialog">取消</button>
        <button class="btn btn-primary" type="button" @click="confirmRename">保存</button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="deleteDialogOpen"
      :title="deleteDialogTitle"
      :message="deleteDialogMessage"
      confirm-text="确认删除"
      cancel-text="取消"
      danger
      @confirm="confirmDelete"
      @cancel="closeDeleteDialog"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { imagesAPI, type GeneratedImage, type ImageModel, type ImageQuality, type ImageSize } from '@/api/images'
import { keysAPI } from '@/api/keys'
import type { ApiKey } from '@/types'
import { maskApiKey } from '@/utils/maskApiKey'

type ConversationMode = 'generate' | 'edit'
type TurnStatus = 'queued' | 'generating' | 'success' | 'error'
type AspectRatioValue = 'auto' | '1:1' | '16:9' | '4:3' | '3:4' | '9:16'
type ResolutionTier = '1K' | '2K' | '4K'

interface StoredReferenceImage {
  name: string
  type: string
  dataUrl: string
}

interface StoredImage {
  id: string
  status: TurnStatus
  b64_json?: string
  url?: string
  revised_prompt?: string
  error?: string
}

interface ImageTurn {
  id: string
  prompt: string
  model: ImageModel
  mode: ConversationMode
  referenceImages: StoredReferenceImage[]
  count: number
  size: ImageSize
  images: StoredImage[]
  createdAt: string
  status: TurnStatus
  error?: string
  promptDeleted?: boolean
  resultsDeleted?: boolean
}

interface ImageConversation {
  id: string
  title: string
  createdAt: string
  updatedAt: string
  turns: ImageTurn[]
}

interface ComposerForm {
  apiKey: string
  selectedApiKeyId: string
  prompt: string
  count: string
  size: ImageSize
  aspectRatio: AspectRatioValue
  resolutionTier: ResolutionTier
  model: ImageModel
}

interface PersistState {
  conversations: ImageConversation[]
  activeConversationId: string | null
  composerReferences: StoredReferenceImage[]
  composerForm: ComposerForm
}

type DeleteTarget =
  | { type: 'one', id: string }
  | { type: 'prompt', turnId: string }
  | { type: 'results', turnId: string }
  | { type: 'all' }

interface LightboxItem {
  id: string
  src: string
  name?: string
  meta?: string
  reference?: StoredReferenceImage
}

interface ApiKeyOption {
  id: string
  key: string
  label: string
}

const STORAGE_KEY = 'sub2api:image-conversations:v1'
const ACTIVE_KEY = 'sub2api:image-active-conversation:v1'
const DRAFT_KEY = 'sub2api:image-draft:v1'
const DB_NAME = 'sub2api-image-workbench'
const DB_STORE = 'image_state'
const DB_STATE_KEY = 'state'
const MANUAL_API_KEY_ID = 'manual'
const DEFAULT_MODEL: ImageModel = 'gpt-image-2'
const modelOptions: ImageModel[] = ['gpt-image-2', 'gpt-image-1.5', 'gpt-image-1']
const aspectRatioOptions: Array<{ value: AspectRatioValue, label: string }> = [
  { value: 'auto', label: '未指定' },
  { value: '1:1', label: '1:1（正方形）' },
  { value: '16:9', label: '16:9（横版）' },
  { value: '4:3', label: '4:3（横版）' },
  { value: '3:4', label: '3:4（竖版）' },
  { value: '9:16', label: '9:16（竖版）' }
]
const baseResolutionOptions: Array<{ value: ResolutionTier, label: string }> = [
  { value: '1K', label: '1K' },
  { value: '2K', label: '2K' },
  { value: '4K', label: '4K' }
]
const legacyResolutionOptions: Array<{ value: ResolutionTier, label: string }> = [
  { value: '1K', label: '1K' },
  { value: '2K', label: '2K' }
]
const imageSizeByAspectAndTier: Record<AspectRatioValue, Record<ResolutionTier, ImageSize>> = {
  auto: {
    '1K': '1024x1024',
    '2K': 'auto',
    '4K': 'auto'
  },
  '1:1': {
    '1K': '1024x1024',
    '2K': '2048x2048',
    '4K': '2880x2880'
  },
  '16:9': {
    '1K': '1152x656',
    '2K': '2048x1152',
    '4K': '3840x2160'
  },
  '4:3': {
    '1K': '1024x768',
    '2K': '2048x1536',
    '4K': '3312x2480'
  },
  '3:4': {
    '1K': '768x1024',
    '2K': '1536x2048',
    '4K': '2480x3312'
  },
  '9:16': {
    '1K': '656x1152',
    '2K': '1152x2048',
    '4K': '2160x3840'
  }
}
const legacyImageSizeByAspect: Record<AspectRatioValue, Record<ResolutionTier, ImageSize>> = {
  auto: {
    '1K': '1024x1024',
    '2K': 'auto',
    '4K': 'auto'
  },
  '1:1': {
    '1K': '1024x1024',
    '2K': '1024x1024',
    '4K': '1024x1024'
  },
  '16:9': {
    '1K': '1536x1024',
    '2K': '1536x1024',
    '4K': '1536x1024'
  },
  '4:3': {
    '1K': '1536x1024',
    '2K': '1536x1024',
    '4K': '1536x1024'
  },
  '3:4': {
    '1K': '1024x1536',
    '2K': '1024x1536',
    '4K': '1024x1536'
  },
  '9:16': {
    '1K': '1024x1536',
    '2K': '1024x1536',
    '4K': '1024x1536'
  }
}
const activeConversationQueueIds = new Set<string>()

const appStore = useAppStore()
const authStore = useAuthStore()
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const scrollRef = ref<HTMLElement | null>(null)
const submitting = ref(false)
const conversations = ref<ImageConversation[]>([])
const selectedConversationId = ref<string | null>(null)
const composerReferences = ref<StoredReferenceImage[]>([])
const renameDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const renameTargetId = ref<string | null>(null)
const deleteTarget = ref<DeleteTarget | null>(null)
const renameValue = ref('')
const hasLoaded = ref(false)
const imageDimensions = ref<Record<string, string>>({})
const lightboxOpen = ref(false)
const lightboxImages = ref<LightboxItem[]>([])
const lightboxIndex = ref(0)
const apiKeysLoading = ref(false)
const userApiKeys = ref<ApiKey[]>([])

const form = reactive<ComposerForm>({
  apiKey: '',
  selectedApiKeyId: MANUAL_API_KEY_ID,
  prompt: '',
  count: '1',
  size: '1024x1024',
  aspectRatio: '1:1',
  resolutionTier: '1K',
  model: DEFAULT_MODEL
})

function createId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function nowIso(): string {
  return new Date().toISOString()
}

function safeNumber(value: string, fallback = 1): number {
  const parsed = Number.parseInt(value, 10)
  if (Number.isNaN(parsed) || parsed <= 0) {
    return fallback
  }
  return Math.min(8, Math.max(1, parsed))
}

function isAspectRatioValue(value: unknown): value is AspectRatioValue {
  return typeof value === 'string' && aspectRatioOptions.some((option) => option.value === value)
}

function isResolutionTier(value: unknown): value is ResolutionTier {
  return value === '1K' || value === '2K' || value === '4K'
}

function isImageSizeValue(value: unknown): value is ImageSize {
  return value === 'auto' || (typeof value === 'string' && /^\d+x\d+$/i.test(value))
}

function inferAspectRatioFromSize(size: ImageSize): AspectRatioValue {
  const normalized = size.toLowerCase()
  if (normalized === 'auto') return 'auto'
  if (normalized === '1024x1024' || normalized === '2048x2048' || normalized === '2880x2880') return '1:1'
  if (normalized === '1536x1024' || normalized === '1152x656' || normalized === '2048x1152' || normalized === '3840x2160') return '16:9'
  if (normalized === '1024x1536' || normalized === '656x1152' || normalized === '1152x2048' || normalized === '2160x3840') return '9:16'
  const [rawWidth, rawHeight] = normalized.split('x')
  const width = Number.parseInt(rawWidth, 10)
  const height = Number.parseInt(rawHeight, 10)
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return '1:1'
  const ratio = width / height
  const candidates: Array<{ value: AspectRatioValue, ratio: number }> = [
    { value: '1:1', ratio: 1 },
    { value: '16:9', ratio: 16 / 9 },
    { value: '4:3', ratio: 4 / 3 },
    { value: '3:4', ratio: 3 / 4 },
    { value: '9:16', ratio: 9 / 16 }
  ]
  return candidates.reduce((best, current) => (Math.abs(current.ratio - ratio) < Math.abs(best.ratio - ratio) ? current : best)).value
}

function inferResolutionTierFromSize(size: ImageSize): ResolutionTier {
  const normalized = size.toLowerCase()
  if (normalized === 'auto') return '2K'
  const [rawWidth, rawHeight] = normalized.split('x')
  const width = Number.parseInt(rawWidth, 10)
  const height = Number.parseInt(rawHeight, 10)
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return '1K'
  const pixels = width * height
  if (pixels > 2560 * 1440) return '4K'
  if (pixels > 1024 * 1024) return '2K'
  return '1K'
}

function resolveImageSize(model: ImageModel, aspectRatio: AspectRatioValue, resolutionTier: ResolutionTier): ImageSize {
  const mapping = model === 'gpt-image-2' ? imageSizeByAspectAndTier : legacyImageSizeByAspect
  return mapping[aspectRatio]?.[resolutionTier] || mapping['1:1']['1K']
}

function imageSizeLabel(size: ImageSize): string {
  if (size === 'auto') {
    return '自动'
  }
  const aspect = aspectRatioOptions.find((option) => option.value === inferAspectRatioFromSize(size))?.label || ''
  return aspect ? `${aspect} · ${size}` : size
}

function normalizeError(error: unknown): string {
  if (error && typeof error === 'object') {
    const maybeAxios = error as { response?: { data?: unknown }, message?: string }
    const data = maybeAxios.response?.data
    if (data && typeof data === 'object') {
      const payload = data as { error?: { message?: string }, message?: string }
      return payload.error?.message || payload.message || maybeAxios.message || '请求失败'
    }
    return maybeAxios.message || '请求失败'
  }
  return '请求失败'
}

function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new Error('读取图片失败'))
    reader.readAsDataURL(file)
  })
}

function dataUrlToFile(dataUrl: string, fileName: string, mimeType?: string): File {
  const [header, content = ''] = dataUrl.split(',', 2)
  const matchedMimeType = header.match(/data:(.*?);base64/)?.[1]
  const binary = atob(content)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return new File([bytes], fileName, { type: mimeType || matchedMimeType || 'image/png' })
}

function buildConversationTitle(prompt: string): string {
  const text = prompt.trim()
  if (!text) {
    return '新会话'
  }
  return text.length > 16 ? `${text.slice(0, 16)}...` : text
}

function toImageSrc(image: StoredImage): string {
  if (image.b64_json) {
    return `data:image/png;base64,${image.b64_json}`
  }
  return image.url || ''
}

function fromGeneratedImage(image: GeneratedImage, index: number): StoredImage {
  return {
    id: `${Date.now()}-${index}`,
    status: 'success',
    b64_json: image.b64_json,
    url: image.url,
    revised_prompt: image.revised_prompt
  }
}

function makeLoadingImages(count: number): StoredImage[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `${createId()}-${index}`,
    status: 'generating'
  }))
}

function normalizeStoredImage(image: StoredImage): StoredImage {
  if (image.status === 'queued' || image.status === 'generating' || image.status === 'success' || image.status === 'error') {
    return image
  }
  return {
    ...image,
    status: image.b64_json || image.url ? 'success' : 'generating'
  }
}

function normalizeReferenceImage(image: StoredReferenceImage): StoredReferenceImage {
  return {
    name: image.name || 'reference.png',
    type: image.type || 'image/png',
    dataUrl: image.dataUrl
  }
}

function deriveTurnStatus(turn: ImageTurn): Pick<ImageTurn, 'status' | 'error'> {
  if (turn.resultsDeleted) {
    return { status: 'success', error: undefined }
  }
  const generatingCount = turn.images.filter((image) => image.status === 'generating' || image.status === 'queued').length
  const failedCount = turn.images.filter((image) => image.status === 'error').length
  const successCount = turn.images.filter((image) => image.status === 'success').length
  if (generatingCount > 0) {
    return { status: turn.status === 'queued' ? 'queued' : 'generating', error: undefined }
  }
  if (failedCount > 0) {
    return { status: 'error', error: `其中 ${failedCount} 张未成功生成` }
  }
  if (successCount > 0 || turn.images.length === 0) {
    return { status: 'success', error: undefined }
  }
  return { status: 'queued', error: undefined }
}

function normalizeTurn(turn: ImageTurn): ImageTurn {
  const images = Array.isArray(turn.images) ? turn.images.map(normalizeStoredImage) : []
  const normalizedSize = isImageSizeValue(turn.size) ? turn.size : '1024x1024'
  const normalized: ImageTurn = {
    id: String(turn.id || createId()),
    prompt: String(turn.prompt || ''),
    model: modelOptions.includes(turn.model) ? turn.model : DEFAULT_MODEL,
    mode: turn.mode === 'edit' ? 'edit' : 'generate',
    referenceImages: Array.isArray(turn.referenceImages) ? turn.referenceImages.map(normalizeReferenceImage).filter((image) => !!image.dataUrl) : [],
    count: Math.max(1, Number(turn.count || images.length || 1)),
    size: normalizedSize,
    images,
    createdAt: String(turn.createdAt || nowIso()),
    status: turn.status === 'queued' || turn.status === 'generating' || turn.status === 'success' || turn.status === 'error' ? turn.status : 'success',
    error: typeof turn.error === 'string' ? turn.error : undefined,
    promptDeleted: turn.promptDeleted === true,
    resultsDeleted: turn.resultsDeleted === true
  }
  const derived = deriveTurnStatus(normalized)
  return { ...normalized, ...derived }
}

function normalizeConversationItem(conversation: ImageConversation): ImageConversation {
  const turns = Array.isArray(conversation.turns) ? conversation.turns.map(normalizeTurn) : []
  const fallbackTime = turns[turns.length - 1]?.createdAt || nowIso()
  return {
    id: String(conversation.id || createId()),
    title: String(conversation.title || turns[0]?.prompt || '新会话'),
    createdAt: String(conversation.createdAt || fallbackTime),
    updatedAt: String(conversation.updatedAt || fallbackTime),
    turns
  }
}

function currentConversation(): ImageConversation | null {
  return conversations.value.find((item) => item.id === selectedConversationId.value) || null
}

const selectedConversation = computed(() => currentConversation())
const activeLightboxImage = computed(() => lightboxImages.value[lightboxIndex.value] || null)
const apiKeyOptions = computed<ApiKeyOption[]>(() => {
  const now = Date.now()
  return userApiKeys.value
    .filter((key) => key.status === 'active')
    .filter((key) => !key.expires_at || new Date(key.expires_at).getTime() > now)
    .filter((key) => Boolean(key.key))
    .map((key) => ({
      id: String(key.id),
      key: key.key,
      label: `${key.name || `Key ${key.id}`} (${maskApiKey(key.key)})`
    }))
})
const selectedApiKeyOption = computed(() => apiKeyOptions.value.find((option) => option.id === form.selectedApiKeyId) || null)
const effectiveApiKey = computed(() => selectedApiKeyOption.value?.key || form.apiKey.trim())
const selectedAspectLabel = computed(() => aspectRatioOptions.find((option) => option.value === form.aspectRatio)?.label || '未指定')
const selectedResolutionLabel = computed(() => form.resolutionTier)
const availableResolutionOptions = computed(() => (form.model === 'gpt-image-2' ? baseResolutionOptions : legacyResolutionOptions))
const resolvedImageSize = computed(() => resolveImageSize(form.model, form.aspectRatio, form.resolutionTier))
const pendingCount = computed(() =>
  conversations.value.reduce(
    (sum, conversation) => sum + conversation.turns.filter((turn) => !turn.resultsDeleted && (turn.status === 'queued' || turn.status === 'generating')).length,
    0,
  ),
)
const deleteDialogTitle = computed(() => {
  switch (deleteTarget.value?.type) {
    case 'all':
      return '清空历史记录'
    case 'prompt':
      return '删除提示词记录'
    case 'results':
      return '删除生成结果'
    case 'one':
      return '删除会话'
    default:
      return '删除'
  }
})
const deleteDialogMessage = computed(() => {
  switch (deleteTarget.value?.type) {
    case 'all':
      return '确认删除全部图片历史记录吗？删除后无法恢复。'
    case 'prompt':
      return '确认删除这条提示词记录吗？对应生成结果会保留。'
    case 'results':
      return '确认删除这条生成结果吗？对应提示词记录会保留。'
    case 'one':
      return '确认删除这条图片对话吗？删除后无法恢复。'
    default:
      return '确认删除吗？'
  }
})

function conversationHasPending(conversation: ImageConversation): boolean {
  return conversation.turns.some((turn) => !turn.resultsDeleted && (turn.status === 'queued' || turn.status === 'generating'))
}

function conversationHasError(conversation: ImageConversation): boolean {
  return conversation.turns.some((turn) => !turn.resultsDeleted && turn.status === 'error')
}

function turnStatusLabel(status: TurnStatus): string {
  switch (status) {
    case 'queued':
      return '排队中'
    case 'generating':
      return '生成中'
    case 'success':
      return '已完成'
    case 'error':
      return '失败'
    default:
      return status
  }
}

function turnStatusClass(status: TurnStatus): string {
  switch (status) {
    case 'queued':
      return 'badge-warning'
    case 'generating':
      return 'badge-warning'
    case 'success':
      return 'badge-success'
    case 'error':
      return 'badge-danger'
    default:
      return 'badge-gray'
  }
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ''
  }
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}

async function persistState(): Promise<void> {
  if (typeof window === 'undefined') {
    return
  }
  const state: PersistState = {
    conversations: conversations.value,
    activeConversationId: selectedConversationId.value,
    composerReferences: composerReferences.value,
    composerForm: { ...form }
  }
  await writePersistedState(state)
  if (selectedConversationId.value) {
    window.localStorage.setItem(ACTIVE_KEY, selectedConversationId.value)
  } else {
    window.localStorage.removeItem(ACTIVE_KEY)
  }
  window.localStorage.setItem(DRAFT_KEY, JSON.stringify({
    apiKey: form.apiKey,
    selectedApiKeyId: form.selectedApiKeyId,
    prompt: form.prompt,
    count: form.count,
    size: form.size,
    aspectRatio: form.aspectRatio,
    resolutionTier: form.resolutionTier,
    model: form.model
  }))
}

function normalizeConversations(items: ImageConversation[]): ImageConversation[] {
  return [...items].map(normalizeConversationItem).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
}

function openImageDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, 1)
    request.onupgradeneeded = () => {
      request.result.createObjectStore(DB_STORE)
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error || new Error('打开图片历史数据库失败'))
  })
}

async function readPersistedState(): Promise<PersistState | null> {
  if (typeof window === 'undefined') {
    return null
  }
  if (typeof indexedDB === 'undefined') {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) as PersistState : null
  }

  const db = await openImageDatabase()
  try {
    const value = await new Promise<PersistState | undefined>((resolve, reject) => {
      const transaction = db.transaction(DB_STORE, 'readonly')
      const request = transaction.objectStore(DB_STORE).get(DB_STATE_KEY)
      request.onsuccess = () => resolve(request.result as PersistState | undefined)
      request.onerror = () => reject(request.error || new Error('读取图片历史失败'))
    })
    if (value) {
      return value
    }
  } finally {
    db.close()
  }

  const legacyRaw = window.localStorage.getItem(STORAGE_KEY)
  return legacyRaw ? JSON.parse(legacyRaw) as PersistState : null
}

async function writePersistedState(state: PersistState): Promise<void> {
  if (typeof window === 'undefined') {
    return
  }
  if (typeof indexedDB === 'undefined') {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
    return
  }

  const db = await openImageDatabase()
  try {
    await new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(DB_STORE, 'readwrite')
      transaction.objectStore(DB_STORE).put(state, DB_STATE_KEY)
      transaction.oncomplete = () => resolve()
      transaction.onerror = () => reject(transaction.error || new Error('保存图片历史失败'))
    })
  } finally {
    db.close()
  }
}

async function removePersistedState(): Promise<void> {
  if (typeof window === 'undefined') {
    return
  }
  localStorage.removeItem(STORAGE_KEY)
  if (typeof indexedDB === 'undefined') {
    return
  }
  const db = await openImageDatabase()
  try {
    await new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(DB_STORE, 'readwrite')
      transaction.objectStore(DB_STORE).delete(DB_STATE_KEY)
      transaction.oncomplete = () => resolve()
      transaction.onerror = () => reject(transaction.error || new Error('清空图片历史失败'))
    })
  } finally {
    db.close()
  }
}

async function loadPersistedState(): Promise<void> {
  if (typeof window === 'undefined') {
    hasLoaded.value = true
    return
  }
  try {
    const parsed = await readPersistedState()
    if (parsed) {
      conversations.value = normalizeConversations(parsed.conversations || [])
      selectedConversationId.value = parsed.activeConversationId || window.localStorage.getItem(ACTIVE_KEY) || null
      composerReferences.value = parsed.composerReferences || []
      if (parsed.composerForm) {
        form.prompt = parsed.composerForm.prompt || ''
        form.apiKey = parsed.composerForm.apiKey || ''
        form.selectedApiKeyId = parsed.composerForm.selectedApiKeyId || MANUAL_API_KEY_ID
        form.count = parsed.composerForm.count || '1'
        form.size = isImageSizeValue(parsed.composerForm.size) ? parsed.composerForm.size : '1024x1024'
        form.aspectRatio = isAspectRatioValue(parsed.composerForm.aspectRatio) ? parsed.composerForm.aspectRatio : inferAspectRatioFromSize(form.size)
        form.resolutionTier = isResolutionTier(parsed.composerForm.resolutionTier) ? parsed.composerForm.resolutionTier : inferResolutionTierFromSize(form.size)
        form.model = parsed.composerForm.model || DEFAULT_MODEL
      }
    } else {
      selectedConversationId.value = window.localStorage.getItem(ACTIVE_KEY)
      const draftRaw = window.localStorage.getItem(DRAFT_KEY)
      if (draftRaw) {
        const draft = JSON.parse(draftRaw) as Partial<ComposerForm>
        form.apiKey = draft.apiKey || ''
        form.selectedApiKeyId = draft.selectedApiKeyId || MANUAL_API_KEY_ID
        form.prompt = draft.prompt || ''
        form.count = draft.count || '1'
        form.size = isImageSizeValue(draft.size) ? draft.size : '1024x1024'
        form.aspectRatio = isAspectRatioValue(draft.aspectRatio) ? draft.aspectRatio : inferAspectRatioFromSize(form.size)
        form.resolutionTier = isResolutionTier(draft.resolutionTier) ? draft.resolutionTier : inferResolutionTierFromSize(form.size)
        form.model = (draft.model as ImageModel) || DEFAULT_MODEL
      }
    }

    if (selectedConversationId.value && !conversations.value.some((item) => item.id === selectedConversationId.value)) {
      selectedConversationId.value = conversations.value[0]?.id || null
    }
  } catch {
    conversations.value = []
    selectedConversationId.value = null
    composerReferences.value = []
  } finally {
    hasLoaded.value = true
  }
}

async function loadUserApiKeys(): Promise<void> {
  if (!authStore.isAuthenticated) {
    return
  }
  apiKeysLoading.value = true
  try {
    const response = await keysAPI.list(1, 100, { status: 'active' })
    userApiKeys.value = response.items || []
    if (form.selectedApiKeyId !== MANUAL_API_KEY_ID && !apiKeyOptions.value.some((option) => option.id === form.selectedApiKeyId)) {
      form.selectedApiKeyId = MANUAL_API_KEY_ID
    }
  } catch (error) {
    console.warn('Failed to load image API keys:', error)
  } finally {
    apiKeysLoading.value = false
  }
}

function createDraftConversation(): void {
  selectedConversationId.value = null
  clearComposer()
  scrollToBottom()
  textareaRef.value?.focus()
}

function selectConversation(id: string): void {
  selectedConversationId.value = id
  scrollToBottom()
}

function scrollToBottom(): void {
  requestAnimationFrame(() => {
    const el = scrollRef.value
    if (!el) {
      return
    }
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  })
}

function ensureConversation(prompt: string): ImageConversation {
  const now = nowIso()
  const existing = selectedConversation.value
  if (existing) {
    return {
      ...existing,
      updatedAt: now,
      turns: [...existing.turns]
    }
  }
  return {
    id: createId(),
    title: buildConversationTitle(prompt),
    createdAt: now,
    updatedAt: now,
    turns: []
  }
}

async function submitTurn(): Promise<void> {
  const prompt = form.prompt.trim()
  if (!prompt) {
    return
  }

  const count = safeNumber(form.count, 1)
  const now = nowIso()
  const conversation = ensureConversation(prompt)
  const mode: ConversationMode = composerReferences.value.length > 0 ? 'edit' : 'generate'
  const size = resolvedImageSize.value
  const turnId = createId()
  const turn: ImageTurn = {
    id: turnId,
    prompt,
    model: form.model,
    mode,
    referenceImages: composerReferences.value.map((item) => ({ ...item })),
    count,
    size,
    images: makeLoadingImages(count),
    createdAt: now,
    status: 'queued'
  }

  const nextConversation: ImageConversation = {
    ...conversation,
    title: conversation.turns.length === 0 ? buildConversationTitle(prompt) : conversation.title,
    updatedAt: now,
    turns: [...conversation.turns, turn]
  }

  conversations.value = normalizeConversations([
    nextConversation,
    ...conversations.value.filter((item) => item.id !== nextConversation.id)
  ])
  selectedConversationId.value = nextConversation.id
  form.prompt = ''
  composerReferences.value = []
  await persistState()
  void runTurn(nextConversation.id, turnId)
}

async function runTurn(conversationId: string, turnId: string): Promise<void> {
  if (activeConversationQueueIds.has(conversationId)) {
    return
  }
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const turn = conversation?.turns.find((item) => item.id === turnId || ((item.status === 'queued' || item.status === 'generating') && item.images.some((image) => image.status === 'generating')))
  if (!conversation || !turn) {
    return
  }

  activeConversationQueueIds.add(conversationId)
  submitting.value = true
  try {
    await updateTurn(conversationId, turn.id, {
      status: 'generating',
      error: undefined
    })

    const referenceFiles = await Promise.all(
      turn.referenceImages.map((image, index) => dataUrlToFile(image.dataUrl, image.name || `reference-${index + 1}.png`, image.type))
    )

    const apiKey = effectiveApiKey.value
    const payload =
      turn.mode === 'edit' && referenceFiles.length > 0
        ? {
            apiKey,
            model: turn.model,
            prompt: turn.prompt,
            size: turn.size,
            quality: 'high' as ImageQuality,
            count: turn.count,
            images: referenceFiles
          }
        : {
            apiKey,
            model: turn.model,
            prompt: turn.prompt,
            size: turn.size,
            quality: 'high' as ImageQuality,
            count: turn.count
          }

    const result = await callImageApi(payload)
    const generatedImages = (result.data || []).map(fromGeneratedImage)
    if (generatedImages.length === 0) {
      throw new Error('接口没有返回图片')
    }

    const latestTurn = conversations.value.find((item) => item.id === conversationId)?.turns.find((item) => item.id === turn.id)
    const generatedQueue = [...generatedImages]
    const images = (latestTurn?.images || turn.images).map((image) => {
      if (image.status !== 'generating') {
        return image
      }
      return generatedQueue.shift() || image
    })
    for (const image of generatedQueue) {
      images.push(image)
    }
    const derived = deriveTurnStatus({ ...turn, images })
    await updateTurn(conversationId, turn.id, {
      ...derived,
      images,
      error: undefined
    })
    appStore.showSuccess('图片生成完成')
  } catch (error) {
    const message = normalizeError(error)
    await updateTurn(conversationId, turn.id, {
      status: 'error',
      error: message,
      images: conversations.value.find((item) => item.id === conversationId)?.turns.find((item) => item.id === turn.id)?.images.map((image) =>
        image.status === 'generating' ? { ...image, status: 'error', error: message } : image
      ) || []
    })
    appStore.showError(message)
  } finally {
    submitting.value = false
    activeConversationQueueIds.delete(conversationId)
    await persistState()
    scrollToBottom()
    const nextTurn = conversations.value
      .find((item) => item.id === conversationId)
      ?.turns.find((item) => !item.resultsDeleted && (item.status === 'queued' || item.status === 'generating') && item.images.some((image) => image.status === 'generating'))
    if (nextTurn) {
      void runTurn(conversationId, nextTurn.id)
    }
  }
}

async function updateTurn(conversationId: string, turnId: string, patch: Partial<ImageTurn>): Promise<void> {
  const next = conversations.value.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation
    }
    const turns = conversation.turns.map((turn) => (turn.id === turnId ? { ...turn, ...patch } : turn))
    return { ...conversation, updatedAt: nowIso(), turns }
  })
  conversations.value = normalizeConversations(next)
}

async function callImageApi(payload: {
  apiKey: string
  model: ImageModel
  prompt: string
  size: ImageSize
  quality: ImageQuality
  count: number
  images?: File[]
}): Promise<{ data?: GeneratedImage[] }> {
  const apiKey = payload.apiKey.trim()
  if (!apiKey) {
    throw new Error('请先输入可用的 API Key')
  }

  if (payload.images && payload.images.length > 0) {
    const formData = new FormData()
    formData.append('model', payload.model)
    formData.append('prompt', payload.prompt)
    formData.append('size', payload.size)
    formData.append('quality', payload.quality)
    formData.append('n', String(payload.count))
    payload.images.forEach((file, index) => {
      formData.append(index === 0 ? 'image' : `image[${index}]`, file, file.name)
    })
    const response = await fetch('/v1/images/edits', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${apiKey}`
      },
      body: formData
    })
    if (!response.ok) {
      throw new Error(await response.text())
    }
    return response.json()
  }

  const response = await imagesAPI.generate({
    apiKey,
    model: payload.model,
    prompt: payload.prompt,
    size: payload.size,
    quality: payload.quality,
    n: payload.count
  })
  return response
}

function imageSrc(image: StoredImage): string {
  return toImageSrc(image)
}

function imageMeta(image: StoredImage): string {
  return [image.b64_json ? formatBase64ImageSize(image.b64_json) : '', imageDimensions.value[image.id] || ''].filter(Boolean).join(' · ')
}

function handleImageLoad(id: string, event: Event): void {
  const image = event.target as HTMLImageElement
  if (image.naturalWidth > 0 && image.naturalHeight > 0) {
    imageDimensions.value = {
      ...imageDimensions.value,
      [id]: `${image.naturalWidth} x ${image.naturalHeight}`
    }
  }
}

function formatBase64ImageSize(base64: string): string {
  const normalized = base64.replace(/\s/g, '')
  const padding = normalized.endsWith('==') ? 2 : normalized.endsWith('=') ? 1 : 0
  const bytes = Math.max(0, Math.floor((normalized.length * 3) / 4) - padding)
  if (bytes >= 1024 * 1024) {
    return `${(bytes / 1024 / 1024).toFixed(2)} MB`
  }
  if (bytes >= 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${bytes} B`
}

function openLightbox(items: LightboxItem[], index: number): void {
  if (items.length === 0) {
    return
  }
  lightboxImages.value = items
  lightboxIndex.value = Math.max(0, Math.min(index, items.length - 1))
  lightboxOpen.value = true
}

function closeLightbox(): void {
  lightboxOpen.value = false
}

function previousLightboxImage(): void {
  if (lightboxImages.value.length <= 1) {
    return
  }
  lightboxIndex.value = (lightboxIndex.value - 1 + lightboxImages.value.length) % lightboxImages.value.length
}

function nextLightboxImage(): void {
  if (lightboxImages.value.length <= 1) {
    return
  }
  lightboxIndex.value = (lightboxIndex.value + 1) % lightboxImages.value.length
}

function referenceToLightboxItem(image: StoredReferenceImage, index: number): LightboxItem {
  return {
    id: `${image.name}-${index}`,
    src: image.dataUrl,
    name: image.name || `参考图 ${index + 1}`,
    reference: image
  }
}

function storedImageToLightboxItem(image: StoredImage, index: number): LightboxItem | null {
  const src = imageSrc(image)
  if (!src || image.status !== 'success') {
    return null
  }
  return {
    id: image.id,
    src,
    name: `结果 ${index + 1}`,
    meta: imageMeta(image),
    reference: {
      name: `result-${Date.now()}-${index + 1}.png`,
      type: 'image/png',
      dataUrl: src
    }
  }
}

function openTurnLightbox(turn: ImageTurn, image: StoredImage): void {
  const items = turn.images
    .map(storedImageToLightboxItem)
    .filter((item): item is LightboxItem => Boolean(item))
  const index = Math.max(0, items.findIndex((item) => item.id === image.id))
  openLightbox(items, index)
}

function openReferenceLightbox(turn: ImageTurn, index: number): void {
  openLightbox(turn.referenceImages.map(referenceToLightboxItem), index)
}

function openComposerLightbox(index: number): void {
  openLightbox(composerReferences.value.map(referenceToLightboxItem), index)
}

function downloadImage(image: StoredImage): void {
  const src = imageSrc(image)
  if (!src) {
    return
  }
  const link = document.createElement('a')
  link.href = src
  link.download = `ai-image-${Date.now()}.png`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

function reuseTurn(turn: ImageTurn): void {
  if (!turn.prompt.trim()) {
    return
  }
  form.prompt = turn.prompt
  form.count = String(turn.count || 1)
  form.size = turn.size
  form.aspectRatio = inferAspectRatioFromSize(turn.size)
  form.resolutionTier = inferResolutionTierFromSize(turn.size)
  form.model = turn.model
  composerReferences.value = turn.referenceImages.map((item) => ({ ...item }))
  selectedConversationId.value = selectedConversation.value?.id || null
  textareaRef.value?.focus()
}

function regenerateTurn(turn: ImageTurn): void {
  if (!selectedConversation.value || !turn.prompt.trim()) {
    return
  }
  const nextTurn: ImageTurn = {
    ...turn,
    id: createId(),
    createdAt: nowIso(),
    status: 'queued',
    error: undefined,
    images: makeLoadingImages(turn.count)
  }
  const nextConversation: ImageConversation = {
    ...selectedConversation.value,
    updatedAt: nowIso(),
    turns: [...selectedConversation.value.turns, nextTurn]
  }
  conversations.value = normalizeConversations([
    nextConversation,
    ...conversations.value.filter((item) => item.id !== nextConversation.id)
  ])
  selectedConversationId.value = nextConversation.id
  void persistState()
  void runTurn(nextConversation.id, nextTurn.id)
}

function retryImage(turnId: string, imageId: string): void {
  const conversation = selectedConversation.value
  if (!conversation) {
    return
  }
  const turn = conversation.turns.find((item) => item.id === turnId)
  if (!turn) {
    return
  }
  if (!turn.prompt.trim()) {
    return
  }
  const retryImageId = `${turnId}-${createId()}`
  const images = turn.images.map((image) => (image.id === imageId ? { id: retryImageId, status: 'generating' as TurnStatus } : image))
  const nextTurn: ImageTurn = {
    ...turn,
    ...deriveTurnStatus({ ...turn, status: 'queued', images }),
    images,
    error: undefined,
    resultsDeleted: false
  }
  const nextConversation: ImageConversation = {
    ...conversation,
    updatedAt: nowIso(),
    turns: conversation.turns.map((item) => (item.id === turnId ? nextTurn : item))
  }
  conversations.value = normalizeConversations([
    nextConversation,
    ...conversations.value.filter((item) => item.id !== nextConversation.id)
  ])
  void persistState()
  void runTurn(nextConversation.id, nextTurn.id)
}

function deletePrompt(turnId: string): void {
  const conversation = selectedConversation.value
  if (!conversation) {
    return
  }
  const nextTurns = conversation.turns
    .map((turn) => {
      if (turn.id !== turnId) {
        return turn
      }
      const nextTurn = {
        ...turn,
        prompt: '',
        promptDeleted: true,
        status: turn.status === 'generating' ? 'error' as TurnStatus : turn.status
      }
      return nextTurn.promptDeleted && nextTurn.resultsDeleted ? null : nextTurn
    })
    .filter((turn): turn is ImageTurn => Boolean(turn))
  if (nextTurns.length === 0) {
    deleteConversation(conversation.id)
    return
  }
  const nextConversation = { ...conversation, updatedAt: nowIso(), turns: nextTurns }
  conversations.value = normalizeConversations([
    nextConversation,
    ...conversations.value.filter((item) => item.id !== nextConversation.id)
  ])
  void persistState()
}

function deleteResults(turnId: string): void {
  const conversation = selectedConversation.value
  if (!conversation) {
    return
  }
  const nextTurns = conversation.turns
    .map((turn) => {
      if (turn.id !== turnId) {
        return turn
      }
      const nextTurn = {
        ...turn,
        images: turn.images.map((image) => ({ id: image.id, status: 'error' as TurnStatus, error: '生成结果已删除' })),
        resultsDeleted: true,
        status: turn.status === 'generating' ? 'error' as TurnStatus : turn.status
      }
      return nextTurn.promptDeleted && nextTurn.resultsDeleted ? null : nextTurn
    })
    .filter((turn): turn is ImageTurn => Boolean(turn))
  if (nextTurns.length === 0) {
    deleteConversation(conversation.id)
    return
  }
  const nextConversation: ImageConversation = { ...conversation, updatedAt: nowIso(), turns: nextTurns }
  conversations.value = normalizeConversations([
    nextConversation,
    ...conversations.value.filter((item) => item.id !== nextConversation.id)
  ])
  void persistState()
}

function continueEdit(image: StoredImage): void {
  const src = imageSrc(image)
  if (!src) {
    return
  }
  composerReferences.value = [
    ...composerReferences.value,
    {
      name: `result-${Date.now()}.png`,
      type: 'image/png',
      dataUrl: src
    }
  ]
  textareaRef.value?.focus()
  appStore.showSuccess('已加入当前参考图，继续输入描述即可编辑')
}

function addReferenceImageToComposer(image: StoredReferenceImage): void {
  composerReferences.value = [...composerReferences.value, { ...image }]
  textareaRef.value?.focus()
  appStore.showSuccess('已加入当前参考图')
}

function pickReferenceImages(): void {
  fileInputRef.value?.click()
}

async function appendReferenceFiles(files: File[]): Promise<void> {
  const imageFiles = files.filter((file) => file.type.startsWith('image/'))
  if (imageFiles.length === 0) {
    return
  }
  try {
    const items = await Promise.all(
      imageFiles.map(async (file) => ({
        name: file.name || `reference-${Date.now()}.png`,
        type: file.type || 'image/png',
        dataUrl: await readFileAsDataUrl(file)
      }))
    )
    composerReferences.value = [...composerReferences.value, ...items]
    appStore.showSuccess('已添加参考图')
  } catch (error) {
    appStore.showError(normalizeError(error))
  }
}

async function handleReferenceFiles(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  await appendReferenceFiles(files)
}

async function handleComposerPaste(event: ClipboardEvent): Promise<void> {
  const files = Array.from(event.clipboardData?.files || []).filter((file) => file.type.startsWith('image/'))
  if (files.length === 0) {
    return
  }
  event.preventDefault()
  await appendReferenceFiles(files)
}

function removeComposerReference(index: number): void {
  composerReferences.value = composerReferences.value.filter((_, current) => current !== index)
}

function clearComposer(): void {
  form.prompt = ''
  composerReferences.value = []
}

function toggleMode(): void {
  if (composerReferences.value.length > 0) {
    composerReferences.value = []
    return
  }
  if (selectedConversation.value?.turns.some((turn) => turn.referenceImages.length > 0)) {
    const lastWithRef = [...selectedConversation.value.turns].reverse().find((turn) => turn.referenceImages.length > 0)
    if (lastWithRef) {
      composerReferences.value = lastWithRef.referenceImages.map((item) => ({ ...item }))
    }
  }
}

function renameConversation(conversation: ImageConversation): void {
  renameTargetId.value = conversation.id
  renameValue.value = conversation.title
  renameDialogOpen.value = true
}

function closeRenameDialog(): void {
  renameDialogOpen.value = false
  renameTargetId.value = null
}

async function confirmRename(): Promise<void> {
  const id = renameTargetId.value
  if (!id) {
    closeRenameDialog()
    return
  }
  const title = renameValue.value.trim() || '新会话'
  conversations.value = normalizeConversations(
    conversations.value.map((item) => (item.id === id ? { ...item, title, updatedAt: nowIso() } : item)),
  )
  closeRenameDialog()
  await persistState()
}

function openDeleteConversation(id: string): void {
  deleteTarget.value = { type: 'one', id }
  deleteDialogOpen.value = true
}

function openDeletePrompt(turnId: string): void {
  deleteTarget.value = { type: 'prompt', turnId }
  deleteDialogOpen.value = true
}

function openDeleteResults(turnId: string): void {
  deleteTarget.value = { type: 'results', turnId }
  deleteDialogOpen.value = true
}

function openClearHistory(): void {
  deleteTarget.value = { type: 'all' }
  deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
  deleteDialogOpen.value = false
  deleteTarget.value = null
}

function deleteConversation(id: string): void {
  conversations.value = conversations.value.filter((item) => item.id !== id)
  if (selectedConversationId.value === id) {
    selectedConversationId.value = conversations.value[0]?.id || null
  }
}

async function clearHistory(): Promise<void> {
  conversations.value = []
  selectedConversationId.value = null
  clearComposer()
  await removePersistedState()
  await persistState()
  appStore.showSuccess('已清空历史记录')
}

async function confirmDelete(): Promise<void> {
  const target = deleteTarget.value
  closeDeleteDialog()
  if (!target) {
    return
  }
  if (target.type === 'all') {
    await clearHistory()
    return
  }
  if (target.type === 'prompt') {
    deletePrompt(target.turnId)
    return
  }
  if (target.type === 'results') {
    deleteResults(target.turnId)
    return
  }
  deleteConversation(target.id)
  await persistState()
}

watch(
  conversations,
  () => {
    void persistState()
  },
  { deep: true }
)

watch(
  selectedConversationId,
  () => {
    scrollToBottom()
    void persistState()
  }
)

watch(
  composerReferences,
  () => {
    void persistState()
  },
  { deep: true }
)

watch(
  () => [form.prompt, form.apiKey, form.selectedApiKeyId, form.count, form.size, form.aspectRatio, form.resolutionTier, form.model],
  () => {
    void persistState()
  }
)

watch(
  () => [form.model, form.aspectRatio, form.resolutionTier] as const,
  () => {
    if (form.model !== 'gpt-image-2' && form.resolutionTier === '4K') {
      form.resolutionTier = '2K'
      return
    }
    form.size = resolvedImageSize.value
  },
  { immediate: true }
)

watch(
  conversations,
  () => {
    for (const conversation of conversations.value) {
      if (activeConversationQueueIds.has(conversation.id)) {
        continue
      }
      const nextTurn = conversation.turns.find(
        (turn) => !turn.resultsDeleted && (turn.status === 'queued' || turn.status === 'generating') && turn.images.some((image) => image.status === 'generating')
      )
      if (nextTurn) {
        void runTurn(conversation.id, nextTurn.id)
      }
    }
  },
  { deep: true }
)

onMounted(async () => {
  await loadPersistedState()
  void loadUserApiKeys()
  textareaRef.value?.focus()
})

onBeforeUnmount(() => {
  void persistState()
})
</script>
