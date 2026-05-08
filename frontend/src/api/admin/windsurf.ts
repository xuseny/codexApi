import { apiClient } from '../client'

export interface WindsurfLoginRequest {
  email?: string
  password?: string
  token?: string
  api_key?: string
  proxy_id?: number
}

export interface WindsurfTokenInfo {
  api_key: string
  api_server_url?: string
  email?: string
  name?: string
  auth_method?: string
  id_token?: string
  refresh_token?: string
  expires_in?: number
  expires_at?: number
  session_token?: string
  auth1_token?: string
  [key: string]: unknown
}

export interface WindsurfRefreshTokenRequest {
  refresh_token: string
  proxy_id?: number
}

export async function login(payload: WindsurfLoginRequest): Promise<WindsurfTokenInfo> {
  const { data } = await apiClient.post<WindsurfTokenInfo>('/admin/windsurf/auth/login', payload)
  return data
}

export async function refreshToken(payload: WindsurfRefreshTokenRequest): Promise<WindsurfTokenInfo> {
  const { data } = await apiClient.post<WindsurfTokenInfo>('/admin/windsurf/auth/refresh-token', payload)
  return data
}

export default { login, refreshToken }
