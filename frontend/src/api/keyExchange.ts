import { apiClient } from './client'
import type {
  APIKeyExchangeKickOfflineRequest,
  APIKeyExchangeKickOfflineResponse,
  APIKeyExchangeResolveRequest,
  APIKeyExchangeResolveResponse
} from '@/types'

export async function resolve(code: string, timezone?: string): Promise<APIKeyExchangeResolveResponse> {
  const payload: APIKeyExchangeResolveRequest = { code }
  if (timezone) {
    payload.timezone = timezone
  }

  const { data } = await apiClient.post<APIKeyExchangeResolveResponse>('/key-exchange/resolve', payload)
  return data
}

export async function kickOffline(code: string): Promise<APIKeyExchangeKickOfflineResponse> {
  const payload: APIKeyExchangeKickOfflineRequest = { code }
  const { data } = await apiClient.post<APIKeyExchangeKickOfflineResponse>('/key-exchange/kick-offline', payload)
  return data
}

export const keyExchangeAPI = {
  resolve,
  kickOffline
}

export default keyExchangeAPI
