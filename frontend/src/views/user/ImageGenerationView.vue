<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="rounded-3xl bg-gradient-to-br from-primary-500 to-fuchsia-600 px-6 py-8 text-white shadow-glow">
        <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <p class="text-sm font-semibold uppercase tracking-[0.2em] text-white/70">Image API</p>
            <h1 class="mt-2 text-3xl font-bold">AI 图片生成</h1>
            <p class="mt-2 max-w-2xl text-sm text-white/80">
              支持文生图、上传参考图结合提示词生成新图，以及一键优化提示词。
            </p>
          </div>
          <div class="rounded-2xl bg-white/15 px-4 py-3 text-sm backdrop-blur-sm">
            <p class="font-medium">当前接口</p>
            <code class="text-white/90">{{ endpointLabel }}</code>
          </div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,460px)_1fr]">
        <div class="card">
          <form class="space-y-5 p-6" @submit.prevent="handleGenerate">
            <div>
              <label for="image-api-key" class="input-label">兑换码或 API Key</label>
              <input
                id="image-api-key"
                v-model="form.apiKey"
                class="input mt-1 w-full"
                type="password"
                autocomplete="off"
                placeholder="sk-..."
                :disabled="busy"
                required
              />
              <p class="input-hint">将作为 Authorization Bearer token 发送，请确认该 Key 有图片和提示词优化权限。</p>
            </div>

            <div>
              <label class="input-label">生成模式</label>
              <div class="mt-2 grid grid-cols-2 gap-2 rounded-2xl bg-gray-100 p-1 dark:bg-dark-800">
                <button
                  type="button"
                  class="rounded-xl px-3 py-2 text-sm font-medium transition-colors"
                  :class="mode === 'generate' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700' : 'text-gray-600 dark:text-dark-300'"
                  :disabled="busy"
                  @click="mode = 'generate'"
                >
                  文生图
                </button>
                <button
                  type="button"
                  class="rounded-xl px-3 py-2 text-sm font-medium transition-colors"
                  :class="mode === 'edit' ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700' : 'text-gray-600 dark:text-dark-300'"
                  :disabled="busy"
                  @click="mode = 'edit'"
                >
                  图生图 / 编辑
                </button>
              </div>
            </div>

            <div v-if="mode === 'edit'" class="space-y-4 rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <div>
                <label for="image-upload" class="input-label">上传参考图片</label>
                <input
                  id="image-upload"
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  multiple
                  class="input mt-1 w-full"
                  :disabled="busy"
                  @change="handleImagesChange"
                />
                <p class="input-hint">支持 1 张或多张图片，后端会以 multipart 方式发送 image / image[n]。</p>
              </div>

              <div v-if="sourcePreviews.length" class="grid grid-cols-3 gap-2">
                <div v-for="preview in sourcePreviews" :key="preview.name" class="overflow-hidden rounded-xl bg-gray-100 dark:bg-dark-800">
                  <img :src="preview.url" :alt="preview.name" class="h-24 w-full object-cover" />
                </div>
              </div>

              <div>
                <div class="flex items-center gap-2">
                  <label for="mask-upload" class="input-label mb-0">可选 mask 图片</label>
                  <div class="group relative inline-flex">
                    <button
                      type="button"
                      class="flex h-5 w-5 items-center justify-center rounded-full border border-gray-300 text-xs font-bold text-gray-500 transition-colors hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:text-dark-400"
                      aria-label="查看 mask 图片说明"
                    >
                      ?
                    </button>
                    <div
                      class="pointer-events-none absolute left-1/2 top-7 z-20 w-80 -translate-x-1/2 rounded-2xl border border-gray-200 bg-white p-4 text-xs leading-5 text-gray-600 opacity-0 shadow-xl transition-opacity group-hover:opacity-100 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-200"
                    >
                      <p class="font-semibold text-gray-800 dark:text-dark-100">mask 图片说明</p>
                      <p class="mt-2">mask 是遮罩图，用来告诉模型哪里可以改、哪里不要动。</p>
                      <p class="mt-1">白色或透明区域通常表示允许修改；黑色或不透明区域通常表示保留原图，具体以模型实现为准。</p>
                      <p class="mt-1">适合只换背景、只换衣服、去掉某个物体、保留人脸或 Logo 等局部重绘场景。</p>
                      <p class="mt-1">不上传 mask 时，模型会根据参考图和提示词整体生成，改动可能更大。</p>
                    </div>
                  </div>
                </div>
                <input
                  id="mask-upload"
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  class="input mt-1 w-full"
                  :disabled="busy"
                  @change="handleMaskChange"
                />
                <p class="input-hint">需要局部编辑时上传 mask，不需要可留空。</p>
              </div>
            </div>

            <div>
              <label for="image-model" class="input-label">图像模型</label>
              <select id="image-model" v-model="form.model" class="input mt-1 w-full" :disabled="busy">
                <option v-for="model in modelOptions" :key="model" :value="model">{{ model }}</option>
              </select>
            </div>

            <div>
              <div class="flex items-center justify-between gap-3">
                <label for="image-prompt" class="input-label">提示词</label>
                <button
                  type="button"
                  class="text-sm font-medium text-primary-600 hover:text-primary-700 disabled:cursor-not-allowed disabled:text-gray-400"
                  :disabled="optimizing || submitting || !canOptimize"
                  @click="handleOptimizePrompt"
                >
                  {{ optimizing ? '优化中...' : '优化提示词' }}
                </button>
              </div>
              <textarea
                id="image-prompt"
                v-model="form.prompt"
                class="input mt-1 min-h-[150px] w-full resize-y"
                placeholder="描述画面主体、风格、构图、光线、色彩、细节；图生图时写清楚保留和修改的内容"
                :disabled="busy"
                required
              />
            </div>

            <div v-if="optimizedPrompt" class="rounded-2xl border border-primary-100 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-sm font-semibold text-primary-700 dark:text-primary-300">优化结果</p>
                  <p class="mt-2 whitespace-pre-wrap text-sm text-gray-700 dark:text-dark-100">{{ optimizedPrompt }}</p>
                </div>
                <button type="button" class="btn btn-secondary shrink-0" :disabled="busy" @click="applyOptimizedPrompt">
                  使用
                </button>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label for="image-size" class="input-label">尺寸</label>
                <select id="image-size" v-model="form.size" class="input mt-1 w-full" :disabled="busy">
                  <option v-for="size in sizeOptions" :key="size" :value="size">{{ size }}</option>
                </select>
              </div>

              <div>
                <label for="image-quality" class="input-label">生图质量</label>
                <select id="image-quality" v-model="form.quality" class="input mt-1 w-full" :disabled="busy">
                  <option v-for="quality in qualityOptions" :key="quality" :value="quality">{{ quality }}</option>
                </select>
              </div>
            </div>

            <div class="rounded-2xl bg-gray-50 p-4 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
              <p class="font-semibold text-gray-800 dark:text-dark-100">值域参考官方文档</p>
              <p class="mt-2">模型：gpt-image-2、gpt-image-1.5、gpt-image-1</p>
              <p>尺寸：auto、1024x1024、1024x1536、1536x1024</p>
              <p>质量：auto、low、medium、high</p>
            </div>

            <button
              type="submit"
              class="btn btn-primary w-full py-3"
              :disabled="submitting || optimizing || !canSubmit"
            >
              <Icon v-if="!submitting" name="sparkles" size="md" class="mr-2" />
              <svg v-else class="-ml-1 mr-2 h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
              {{ submitting ? '生成中...' : '生成图片' }}
            </button>
          </form>
        </div>

        <div class="card min-h-[520px] p-6">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">生成结果</h2>
              <p class="text-sm text-gray-500 dark:text-dark-400">支持预览、打开和下载返回图片。</p>
            </div>
            <button v-if="imageSrc" type="button" class="btn btn-secondary" @click="downloadImage">
              下载
            </button>
          </div>

          <div
            v-if="!imageSrc"
            class="flex min-h-[420px] flex-col items-center justify-center rounded-3xl border-2 border-dashed border-gray-200 bg-gray-50 text-center dark:border-dark-700 dark:bg-dark-800/60"
          >
            <Icon name="sparkles" size="xl" class="text-gray-400 dark:text-dark-500" />
            <p class="mt-4 text-sm font-medium text-gray-600 dark:text-dark-300">还没有图片</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">填写左侧表单后点击生成。</p>
          </div>

          <div v-else class="space-y-4">
            <div class="overflow-hidden rounded-3xl bg-gray-100 dark:bg-dark-800">
              <img :src="imageSrc" alt="Generated image" class="max-h-[720px] w-full object-contain" />
            </div>

            <div v-if="revisedPrompt" class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-800">
              <p class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-400">Revised prompt</p>
              <p class="mt-2 text-sm text-gray-700 dark:text-dark-200">{{ revisedPrompt }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { imagesAPI, type ImageModel, type ImageQuality, type ImageSize } from '@/api/images'
import { useAppStore } from '@/stores/app'

type GenerationMode = 'generate' | 'edit'

interface PreviewItem {
  name: string
  url: string
}

const modelOptions: ImageModel[] = ['gpt-image-2', 'gpt-image-1.5', 'gpt-image-1']
const sizeOptions: ImageSize[] = ['auto', '1024x1024', '1024x1536', '1536x1024']
const qualityOptions: ImageQuality[] = ['auto', 'low', 'medium', 'high']

const appStore = useAppStore()
const mode = ref<GenerationMode>('generate')
const submitting = ref(false)
const optimizing = ref(false)
const imageSrc = ref('')
const revisedPrompt = ref('')
const optimizedPrompt = ref('')
const sourceImages = ref<File[]>([])
const maskImage = ref<File | null>(null)
const sourcePreviews = ref<PreviewItem[]>([])

const form = reactive({
  apiKey: '',
  model: 'gpt-image-2' as ImageModel,
  prompt: '',
  size: '1024x1024' as ImageSize,
  quality: 'high' as ImageQuality
})

const busy = computed(() => submitting.value || optimizing.value)
const endpointLabel = computed(() => mode.value === 'edit' ? 'POST /v1/images/edits' : 'POST /v1/images/generations')
const canOptimize = computed(() => form.apiKey.trim().length > 0 && form.prompt.trim().length > 0)
const canSubmit = computed(() => {
  const baseReady = form.apiKey.trim().length > 0 && form.prompt.trim().length > 0
  return mode.value === 'edit' ? baseReady && sourceImages.value.length > 0 : baseReady
})

function revokePreviews(): void {
  sourcePreviews.value.forEach((preview) => URL.revokeObjectURL(preview.url))
}

function handleImagesChange(event: Event): void {
  const input = event.target as HTMLInputElement
  revokePreviews()
  sourceImages.value = Array.from(input.files || [])
  sourcePreviews.value = sourceImages.value.map((file) => ({
    name: `${file.name}-${file.lastModified}`,
    url: URL.createObjectURL(file)
  }))
}

function handleMaskChange(event: Event): void {
  const input = event.target as HTMLInputElement
  maskImage.value = input.files?.[0] || null
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

async function handleGenerate(): Promise<void> {
  if (!canSubmit.value || submitting.value) {
    return
  }

  submitting.value = true
  revisedPrompt.value = ''

  try {
    const payload = {
      apiKey: form.apiKey.trim(),
      model: form.model,
      prompt: form.prompt.trim(),
      size: form.size,
      quality: form.quality
    }
    const result = mode.value === 'edit'
      ? await imagesAPI.edit({ ...payload, images: sourceImages.value, mask: maskImage.value })
      : await imagesAPI.generate(payload)

    const image = result.data?.[0]
    if (!image?.b64_json && !image?.url) {
      throw new Error('接口未返回图片数据')
    }

    imageSrc.value = image.b64_json ? `data:image/png;base64,${image.b64_json}` : image.url || ''
    revisedPrompt.value = image.revised_prompt || ''
    appStore.showSuccess('图片生成成功')
  } catch (error) {
    appStore.showError(normalizeError(error))
  } finally {
    submitting.value = false
  }
}

async function handleOptimizePrompt(): Promise<void> {
  if (!canOptimize.value || optimizing.value) {
    return
  }

  optimizing.value = true
  try {
    const optimized = await imagesAPI.optimizePrompt({
      apiKey: form.apiKey.trim(),
      prompt: form.prompt.trim()
    })
    if (!optimized) {
      throw new Error('提示词优化接口未返回内容')
    }
    optimizedPrompt.value = optimized
    appStore.showSuccess('提示词优化完成')
  } catch (error) {
    appStore.showError(normalizeError(error))
  } finally {
    optimizing.value = false
  }
}

function applyOptimizedPrompt(): void {
  if (optimizedPrompt.value) {
    form.prompt = optimizedPrompt.value
  }
}

function downloadImage(): void {
  if (!imageSrc.value) {
    return
  }

  const link = document.createElement('a')
  link.href = imageSrc.value
  link.download = `generated-${Date.now()}.png`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

onBeforeUnmount(() => {
  revokePreviews()
})
</script>
