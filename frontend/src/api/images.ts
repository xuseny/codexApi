import axios from 'axios'

const DEFAULT_PROMPT_OPTIMIZER_MODEL = 'gpt-4.1-mini'

const imageGatewayClient = axios.create({
  baseURL: ''
})

export type ImageModel = 'gpt-image-2' | 'gpt-image-1.5' | 'gpt-image-1'
export type ImageQuality = 'auto' | 'low' | 'medium' | 'high'
export type ImageSize = 'auto' | '1024x1024' | '1024x1536' | '1536x1024'

export interface GenerateImageRequest {
  apiKey: string
  model: ImageModel
  prompt: string
  size: ImageSize
  quality: ImageQuality
}

export interface EditImageRequest extends GenerateImageRequest {
  images: File[]
  mask?: File | null
}

export interface OptimizePromptRequest {
  apiKey: string
  prompt: string
  model?: string
}

export interface GeneratedImage {
  b64_json?: string
  url?: string
  revised_prompt?: string
}

interface ImagesGenerationResponse {
  created?: number
  data?: GeneratedImage[]
}

interface ChatCompletionResponse {
  choices?: Array<{
    message?: {
      content?: string | Array<{ type?: string; text?: string }>
    }
  }>
}

function authHeaders(apiKey: string): Record<string, string> {
  return {
    Authorization: `Bearer ${apiKey}`
  }
}

function appendImageFiles(formData: FormData, images: File[]): void {
  images.forEach((image, index) => {
    formData.append(index === 0 ? 'image' : `image[${index}]`, image)
  })
}

function extractMessageContent(response: ChatCompletionResponse): string {
  const content = response.choices?.[0]?.message?.content
  if (typeof content === 'string') {
    return content.trim()
  }
  if (Array.isArray(content)) {
    return content
      .map((part) => part.text || '')
      .join('\n')
      .trim()
  }
  return ''
}

export const imagesAPI = {
  async generate(payload: GenerateImageRequest): Promise<ImagesGenerationResponse> {
    const { apiKey, ...body } = payload
    const response = await imageGatewayClient.post<ImagesGenerationResponse>('/v1/images/generations', body, {
      headers: {
        ...authHeaders(apiKey),
        'Content-Type': 'application/json'
      }
    })

    return response.data
  },

  async edit(payload: EditImageRequest): Promise<ImagesGenerationResponse> {
    const formData = new FormData()
    formData.append('model', payload.model)
    formData.append('prompt', payload.prompt)
    formData.append('size', payload.size)
    formData.append('quality', payload.quality)
    appendImageFiles(formData, payload.images)
    if (payload.mask) {
      formData.append('mask', payload.mask)
    }

    const response = await imageGatewayClient.post<ImagesGenerationResponse>('/v1/images/edits', formData, {
      headers: authHeaders(payload.apiKey)
    })

    return response.data
  },

  async optimizePrompt(payload: OptimizePromptRequest): Promise<string> {
    const response = await imageGatewayClient.post<ChatCompletionResponse>(
      '/v1/chat/completions',
      {
        model: payload.model || DEFAULT_PROMPT_OPTIMIZER_MODEL,
        messages: [
          {
            role: 'system',
            content:
              '你是专业 AI 图片提示词优化助手。请把用户的中文或英文想法优化成适合 GPT Image 模型的高质量提示词。只输出优化后的提示词，不要解释。保留用户明确要求，不添加文字水印、签名或无关主体。'
          },
          {
            role: 'user',
            content: payload.prompt
          }
        ],
        temperature: 0.7
      },
      {
        headers: {
          ...authHeaders(payload.apiKey),
          'Content-Type': 'application/json'
        }
      }
    )

    return extractMessageContent(response.data)
  }
}
