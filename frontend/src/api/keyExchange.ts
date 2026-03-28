import { apiClient } from './client'
import type { APIKeyExchangeResolveRequest, APIKeyExchangeResolveResponse } from '@/types'

export async function resolve(code: string, timezone?: string): Promise<APIKeyExchangeResolveResponse> {
  const payload: APIKeyExchangeResolveRequest = { code }
  if (timezone) {
    payload.timezone = timezone
  }

  const { data } = await apiClient.post<APIKeyExchangeResolveResponse>('/key-exchange/resolve', payload)
  return data
}

export const keyExchangeAPI = {
  resolve
}

export default keyExchangeAPI
