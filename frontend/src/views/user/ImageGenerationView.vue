<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="rounded-3xl bg-gradient-to-br from-primary-500 to-fuchsia-600 px-6 py-8 text-white shadow-glow">
        <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <p class="text-sm font-semibold uppercase tracking-[0.2em] text-white/70">Image API</p>
            <h1 class="mt-2 text-3xl font-bold">AI 图片生成</h1>
            <p class="mt-2 max-w-2xl text-sm text-white/80">
              输入兑换码或 API Key，选择 GPT Image 模型、尺寸和质量后生成图片。
            </p>
          </div>
          <div class="rounded-2xl bg-white/15 px-4 py-3 text-sm backdrop-blur-sm">
            <p class="font-medium">接口</p>
            <code class="text-white/90">POST /v1/images/generations</code>
          </div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,420px)_1fr]">
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
                :disabled="submitting"
                required
              />
              <p class="input-hint">将作为 Authorization Bearer token 发送。</p>
            </div>

            <div>
              <label for="image-model" class="input-label">图像模型</label>
              <select id="image-model" v-model="form.model" class="input mt-1 w-full" :disabled="submitting">
                <option v-for="model in modelOptions" :key="model" :value="model">{{ model }}</option>
              </select>
            </div>

            <div>
              <label for="image-prompt" class="input-label">提示词</label>
              <textarea
                id="image-prompt"
                v-model="form.prompt"
                class="input mt-1 min-h-[150px] w-full resize-y"
                placeholder="描述你想生成的画面、风格、主体、构图和细节"
                :disabled="submitting"
                required
              />
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label for="image-size" class="input-label">尺寸</label>
                <select id="image-size" v-model="form.size" class="input mt-1 w-full" :disabled="submitting">
                  <option v-for="size in sizeOptions" :key="size" :value="size">{{ size }}</option>
                </select>
              </div>

              <div>
                <label for="image-quality" class="input-label">生图质量</label>
                <select id="image-quality" v-model="form.quality" class="input mt-1 w-full" :disabled="submitting">
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
              :disabled="submitting || !canSubmit"
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
import { computed, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { imagesAPI, type ImageModel, type ImageQuality, type ImageSize } from '@/api/images'
import { useAppStore } from '@/stores/app'

const modelOptions: ImageModel[] = ['gpt-image-2', 'gpt-image-1.5', 'gpt-image-1']
const sizeOptions: ImageSize[] = ['auto', '1024x1024', '1024x1536', '1536x1024']
const qualityOptions: ImageQuality[] = ['auto', 'low', 'medium', 'high']

const appStore = useAppStore()
const submitting = ref(false)
const imageSrc = ref('')
const revisedPrompt = ref('')

const form = reactive({
  apiKey: '',
  model: 'gpt-image-2' as ImageModel,
  prompt: '',
  size: '1024x1024' as ImageSize,
  quality: 'high' as ImageQuality
})

const canSubmit = computed(() => form.apiKey.trim().length > 0 && form.prompt.trim().length > 0)

function normalizeError(error: unknown): string {
  if (error && typeof error === 'object') {
    const maybeAxios = error as { response?: { data?: unknown }, message?: string }
    const data = maybeAxios.response?.data
    if (data && typeof data === 'object') {
      const payload = data as { error?: { message?: string }, message?: string }
      return payload.error?.message || payload.message || maybeAxios.message || '图片生成失败'
    }
    return maybeAxios.message || '图片生成失败'
  }
  return '图片生成失败'
}

async function handleGenerate(): Promise<void> {
  if (!canSubmit.value || submitting.value) {
    return
  }

  submitting.value = true
  revisedPrompt.value = ''

  try {
    const result = await imagesAPI.generate({
      apiKey: form.apiKey.trim(),
      model: form.model,
      prompt: form.prompt.trim(),
      size: form.size,
      quality: form.quality
    })

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
</script>
