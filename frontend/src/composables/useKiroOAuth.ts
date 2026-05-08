import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { KiroDeviceAuthRequest, KiroDeviceAuthResponse, KiroTokenInfo } from '@/api/admin/kiro'

export function useKiroOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const verificationUri = ref('')
  const userCode = ref('')
  const sessionId = ref('')
  const region = ref('')
  const authMethod = ref('')
  const loading = ref(false)
  const error = ref('')

  const resetState = () => {
    authUrl.value = ''
    verificationUri.value = ''
    userCode.value = ''
    sessionId.value = ''
    region.value = ''
    authMethod.value = ''
    loading.value = false
    error.value = ''
  }

  const applyDeviceAuthResult = (result: KiroDeviceAuthResponse) => {
    authUrl.value = result.verification_uri_complete || result.verification_uri || ''
    verificationUri.value = result.verification_uri || ''
    userCode.value = result.user_code || ''
    sessionId.value = result.session_id || ''
    region.value = result.region || ''
    authMethod.value = result.auth_method || ''
  }

  const startDeviceAuth = async (payload: {
    startUrl?: string
    region?: string
    proxyId?: number | null
  }): Promise<boolean> => {
    loading.value = true
    error.value = ''
    authUrl.value = ''
    verificationUri.value = ''
    userCode.value = ''
    sessionId.value = ''

    try {
      const req: KiroDeviceAuthRequest = {}
      if (payload.startUrl?.trim()) req.start_url = payload.startUrl.trim()
      if (payload.region?.trim()) req.region = payload.region.trim()
      if (payload.proxyId) req.proxy_id = payload.proxyId
      const result = await adminAPI.kiro.startDeviceAuth(req)
      applyDeviceAuthResult(result)
      return true
    } catch (err: any) {
      error.value =
        err.response?.data?.detail || err.response?.data?.message || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const exchangeDeviceCode = async (): Promise<KiroTokenInfo | null> => {
    if (!sessionId.value) {
      error.value = t('admin.accounts.oauth.authFailed')
      return null
    }

    loading.value = true
    error.value = ''

    try {
      return await adminAPI.kiro.exchangeDeviceCode({ session_id: sessionId.value })
    } catch (err: any) {
      error.value =
        err.response?.data?.detail || err.response?.data?.message || t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: KiroTokenInfo): Record<string, unknown> => {
    const credentials: Record<string, unknown> = {
      access_token: tokenInfo.access_token,
      refresh_token: tokenInfo.refresh_token,
      expires_at: tokenInfo.expires_at,
      client_id: tokenInfo.client_id,
      client_secret: tokenInfo.client_secret,
      region: tokenInfo.region,
      auth_method: tokenInfo.auth_method,
      start_url: tokenInfo.start_url
    }
    if (tokenInfo.profile_arn) credentials.profile_arn = tokenInfo.profile_arn
    return Object.fromEntries(
      Object.entries(credentials).filter(([, value]) => value !== undefined && value !== '')
    )
  }

  return {
    authUrl,
    verificationUri,
    userCode,
    sessionId,
    region,
    authMethod,
    loading,
    error,
    resetState,
    startDeviceAuth,
    exchangeDeviceCode,
    buildCredentials
  }
}
