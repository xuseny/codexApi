import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { WindsurfLoginRequest, WindsurfTokenInfo } from '@/api/admin/windsurf'

export type WindsurfLoginMode = 'token' | 'email' | 'api_key'

export function useWindsurfAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const loading = ref(false)
  const error = ref('')

  const resetState = () => {
    loading.value = false
    error.value = ''
  }

  const login = async (payload: WindsurfLoginRequest): Promise<WindsurfTokenInfo | null> => {
    loading.value = true
    error.value = ''
    try {
      return await adminAPI.windsurf.login(payload)
    } catch (err: any) {
      error.value =
        err.response?.data?.detail ||
        err.response?.data?.message ||
        err.message ||
        t('admin.accounts.oauth.authFailed')
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: WindsurfTokenInfo): Record<string, unknown> => {
    const credentials: Record<string, unknown> = {
      api_key: tokenInfo.api_key,
      api_server_url: tokenInfo.api_server_url,
      email: tokenInfo.email,
      name: tokenInfo.name,
      auth_method: tokenInfo.auth_method,
      id_token: tokenInfo.id_token,
      refresh_token: tokenInfo.refresh_token,
      expires_at: tokenInfo.expires_at,
      session_token: tokenInfo.session_token,
      auth1_token: tokenInfo.auth1_token,
      windsurf_builtin: true,
      windsurf_transport: 'language_server'
    }
    return Object.fromEntries(
      Object.entries(credentials).filter(([, value]) => value !== undefined && value !== '')
    )
  }

  return {
    loading,
    error,
    resetState,
    login,
    buildCredentials
  }
}
