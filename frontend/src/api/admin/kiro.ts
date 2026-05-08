import { apiClient } from '../client'

export interface KiroDeviceAuthRequest {
  start_url?: string
  region?: string
  proxy_id?: number
}

export interface KiroDeviceAuthResponse {
  verification_uri?: string
  verification_uri_complete: string
  user_code?: string
  session_id: string
  region: string
  auth_method: string
  expires_in?: number
  interval?: number
}

export interface KiroExchangeDeviceCodeRequest {
  session_id: string
}

export interface KiroTokenInfo {
  access_token?: string
  refresh_token?: string
  expires_in?: number
  expires_at?: number
  client_id?: string
  client_secret?: string
  region?: string
  auth_method?: string
  start_url?: string
  profile_arn?: string
  [key: string]: unknown
}

export interface KiroRefreshTokenRequest {
  refresh_token: string
  client_id: string
  client_secret: string
  region?: string
  auth_method?: string
  start_url?: string
  profile_arn?: string
  proxy_id?: number
}

export async function startDeviceAuth(
  payload: KiroDeviceAuthRequest
): Promise<KiroDeviceAuthResponse> {
  const { data } = await apiClient.post<KiroDeviceAuthResponse>(
    '/admin/kiro/oauth/device-auth',
    payload
  )
  return data
}

export async function exchangeDeviceCode(
  payload: KiroExchangeDeviceCodeRequest
): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>(
    '/admin/kiro/oauth/exchange-device-code',
    payload
  )
  return data
}

export async function refreshKiroToken(
  payload: KiroRefreshTokenRequest
): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>(
    '/admin/kiro/oauth/refresh-token',
    payload
  )
  return data
}

export default { startDeviceAuth, exchangeDeviceCode, refreshKiroToken }
