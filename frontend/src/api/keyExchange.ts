import { publicApiClient } from './client'
import type {
  APIKeyExchangeQuotaRedeemRequest,
  APIKeyExchangeQuotaRedeemResponse,
  APIKeyExchangeResolveRequest,
  APIKeyExchangeResolveResponse
} from '@/types'

export async function resolve(code: string, timezone?: string): Promise<APIKeyExchangeResolveResponse> {
  const payload: APIKeyExchangeResolveRequest = { code }
  if (timezone) {
    payload.timezone = timezone
  }

  const { data } = await publicApiClient.post<APIKeyExchangeResolveResponse>('/key-exchange/resolve', payload)
  return data
}

export async function redeemQuota(
  exchangeCode: string,
  redeemCode: string,
  timezone?: string
): Promise<APIKeyExchangeQuotaRedeemResponse> {
  const payload: APIKeyExchangeQuotaRedeemRequest = {
    exchange_code: exchangeCode,
    redeem_code: redeemCode
  }
  if (timezone) {
    payload.timezone = timezone
  }

  const { data } = await publicApiClient.post<APIKeyExchangeQuotaRedeemResponse>('/key-exchange/redeem-quota', payload)
  return data
}

export const keyExchangeAPI = {
  resolve,
  redeemQuota
}

export default keyExchangeAPI
