import axios from 'axios'

const imageGatewayClient = axios.create({
  baseURL: '',
  timeout: 300000,
  headers: {
    'Content-Type': 'application/json'
  }
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

export interface GeneratedImage {
  b64_json?: string
  url?: string
  revised_prompt?: string
}

interface ImagesGenerationResponse {
  created?: number
  data?: GeneratedImage[]
}

export const imagesAPI = {
  async generate(payload: GenerateImageRequest): Promise<ImagesGenerationResponse> {
    const { apiKey, ...body } = payload
    const response = await imageGatewayClient.post<ImagesGenerationResponse>('/v1/images/generations', body, {
      headers: {
        Authorization: `Bearer ${apiKey}`
      }
    })

    return response.data
  }
}
