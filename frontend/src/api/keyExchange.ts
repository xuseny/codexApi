import { publicApiClient } from './client'
import type {
  APIKeyExchangeUsageLog,
  APIKeyExchangeQuotaRedeemRequest,
  APIKeyExchangeQuotaRedeemResponse,
  APIKeyExchangeResolveRequest,
  APIKeyExchangeResolveResponse,
  PaginatedResponse
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

export async function listUsageLogs(
  code: string,
  page: number = 1,
  pageSize: number = 10
): Promise<PaginatedResponse<APIKeyExchangeUsageLog>> {
  const { data } = await publicApiClient.post<PaginatedResponse<APIKeyExchangeUsageLog>>('/key-exchange/usage-logs', {
    code,
    page,
    page_size: pageSize
  })
  return data
}

export const keyExchangeAPI = {
  resolve,
  redeemQuota,
  listUsageLogs
}

export default keyExchangeAPI
