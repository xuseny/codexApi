import axios from 'axios'

const imageGatewayClient = axios.create({
  baseURL: ''
})

export type ImageModel = 'gpt-image-2' | 'gpt-image-1.5' | 'gpt-image-1'
export type ImageQuality = 'auto' | 'low' | 'medium' | 'high'
export type ImageSize = string

export interface GenerateImageRequest {
  apiKey: string
  model: ImageModel
  prompt: string
  size: ImageSize
  quality: ImageQuality
  n?: number
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

function authHeaders(apiKey: string): Record<string, string> {
  return {
    Authorization: `Bearer ${apiKey}`
  }
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
  }
}
