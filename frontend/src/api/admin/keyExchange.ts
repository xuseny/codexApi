import { apiClient } from '../client'
import type {
  APIKeyExchangeCode,
  APIKeyExchangeCodeStatus,
  GenerateAPIKeyExchangeCodesRequest,
  PaginatedResponse
} from '@/types'

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: APIKeyExchangeCodeStatus | ''
    search?: string
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<APIKeyExchangeCode>> {
  const { data } = await apiClient.get<PaginatedResponse<APIKeyExchangeCode>>('/admin/key-exchange-codes', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

export async function getById(id: number): Promise<APIKeyExchangeCode> {
  const { data } = await apiClient.get<APIKeyExchangeCode>(`/admin/key-exchange-codes/${id}`)
  return data
}

export async function generate(payload: GenerateAPIKeyExchangeCodesRequest): Promise<APIKeyExchangeCode[]> {
  const { data } = await apiClient.post<APIKeyExchangeCode[]>('/admin/key-exchange-codes/generate', payload)
  return data
}

export async function deleteCode(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/key-exchange-codes/${id}`)
  return data
}

export async function batchDelete(ids: number[]): Promise<{
  deleted: number
  message: string
}> {
  const { data } = await apiClient.post<{
    deleted: number
    message: string
  }>('/admin/key-exchange-codes/batch-delete', { ids })
  return data
}

export const keyExchangeAPI = {
  list,
  getById,
  generate,
  delete: deleteCode,
  batchDelete
}

export default keyExchangeAPI
