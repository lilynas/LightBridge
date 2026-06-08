import { apiClient } from '../client'

export interface UITheme {
  id: string
  name: string
  version: string
  source: string
  entry_css: string
  preview?: string
  manifest: Record<string, unknown>
  config: Record<string, unknown>
  active: boolean
  created_at: string
  updated_at: string
}

interface ThemeListResponse {
  themes: UITheme[]
}

interface ThemeInstallResponse {
  theme: UITheme
}

function unwrap<T>(resp: { data: { data: T } }): T {
  return resp.data.data
}

export async function listThemes(): Promise<UITheme[]> {
  const resp = await apiClient.get<{ data: ThemeListResponse }>('/admin/ui-themes')
  return unwrap(resp).themes || []
}

export async function uploadTheme(file: File, replace = false): Promise<UITheme> {
  const form = new FormData()
  form.append('file', file)
  const resp = await apiClient.post<{ data: ThemeInstallResponse }>(
    `/admin/ui-themes/upload${replace ? '?replace=true' : ''}`,
    form,
    { headers: { 'Content-Type': 'multipart/form-data' } },
  )
  return unwrap(resp).theme
}

export async function importGitHubTheme(url: string, replace = false): Promise<UITheme> {
  const resp = await apiClient.post<{ data: ThemeInstallResponse }>('/admin/ui-themes/import-github', { url, replace })
  return unwrap(resp).theme
}

export async function activateTheme(id: string): Promise<UITheme> {
  const resp = await apiClient.put<{ data: UITheme }>(`/admin/ui-themes/${encodeURIComponent(id)}/activate`)
  return unwrap(resp)
}

export async function deactivateTheme(id: string): Promise<UITheme> {
  const resp = await apiClient.put<{ data: UITheme }>(`/admin/ui-themes/${encodeURIComponent(id)}/deactivate`)
  return unwrap(resp)
}

export async function updateThemeConfig(id: string, config: Record<string, unknown>): Promise<UITheme> {
  const resp = await apiClient.put<{ data: UITheme }>(`/admin/ui-themes/${encodeURIComponent(id)}/config`, { config })
  return unwrap(resp)
}

export async function deleteTheme(id: string): Promise<void> {
  await apiClient.delete(`/admin/ui-themes/${encodeURIComponent(id)}`)
}

export default {
  listThemes,
  uploadTheme,
  importGitHubTheme,
  activateTheme,
  deactivateTheme,
  updateThemeConfig,
  deleteTheme,
}
