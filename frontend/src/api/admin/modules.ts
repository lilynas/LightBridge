import { apiClient } from '../client'

export type ModuleStatus = 'installed' | 'enabled' | 'disabled' | 'failed' | 'uninstalled' | 'purged'

export interface ModuleFrontendRoute {
  path: string
  title: string
  exposedModule: string
  requiresAdmin?: boolean
}

export interface ModuleFrontendMenu {
  title: string
  path: string
  group?: string
  order?: number
}

export interface ModuleFrontendAccountForm {
  providerId: string
  providerName?: string
  moduleId?: string
  moduleName?: string
  moduleVersion?: string
  exposedModule: string
  remoteEntry?: string
}

export interface ModuleFrontendSpec {
  kind: 'vite-remote-esm'
  entry: string
  routes?: ModuleFrontendRoute[]
  menu?: ModuleFrontendMenu[]
  accountForms?: ModuleFrontendAccountForm[]
}

export interface ModuleManifest {
  apiVersion: string
  id: string
  name: string
  type: string
  version: string
  description?: string
  capabilities: string[]
  permissions?: Record<string, string[]>
  frontend?: ModuleFrontendSpec
}

export interface InstalledModule {
  id: string
  name: string
  type: string
  version: string
  status: ModuleStatus
  install_path: string
  manifest: ModuleManifest
  installed_at: string
  enabled_at?: string
  last_error?: string
}

export interface MarketplaceModulesResult {
  modules: MarketplaceModule[]
}

export interface MarketplaceModule {
  id: string
  version: string
  type: string
  name?: string
  description?: string
  downloadUrl: string
  sha256?: string
  signature?: string
  core: string
  capabilities?: string[]
  permissions?: Record<string, string[]>
  installedStatus?: ModuleStatus
  installedVersion?: string
}

export type InstallModuleRequest =
  | { archive_path: string; module_id?: never; version?: never }
  | { module_id: string; version: string; archive_path?: never }

export interface ModuleProviderAdapterStatus {
  id: string
  status: 'registered'
}

export interface ModulePermissionRecord {
  module_id: string
  permission_type: string
  permission_value: string
  approved: boolean
  approved_at?: string
  created_at: string
}

export interface ModulePermissionStatus {
  permissions: ModulePermissionRecord[]
  approved: boolean
}

export async function listInstalled(): Promise<InstalledModule[]> {
  const { data } = await apiClient.get<InstalledModule[]>('/admin/modules/installed')
  return data
}

export async function listProviderAdapters(): Promise<ModuleProviderAdapterStatus[]> {
  const { data } = await apiClient.get<ModuleProviderAdapterStatus[]>('/admin/modules/provider-adapters')
  return data
}

export async function listProviderAccountForms(): Promise<ModuleFrontendAccountForm[]> {
  const { data } = await apiClient.get<ModuleFrontendAccountForm[]>('/admin/modules/provider-account-forms')
  return data
}

export async function listMarketplace(): Promise<MarketplaceModulesResult> {
  const { data } = await apiClient.get<MarketplaceModulesResult>('/admin/modules/marketplace')
  return data
}

export async function install(request: InstallModuleRequest): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>('/admin/modules/install', request)
  return data
}

export async function installArchive(archivePath: string): Promise<InstalledModule> {
  return install({ archive_path: archivePath })
}

export async function installFromMarketplace(moduleId: string, version: string): Promise<InstalledModule> {
  return install({ module_id: moduleId, version })
}

export async function permissions(id: string): Promise<ModulePermissionStatus> {
  const { data } = await apiClient.get<ModulePermissionStatus>(`/admin/modules/${encodeURIComponent(id)}/permissions`)
  return data
}

export async function approvePermissions(id: string): Promise<ModulePermissionStatus> {
  const { data } = await apiClient.post<ModulePermissionStatus>(`/admin/modules/${encodeURIComponent(id)}/permissions/approve`)
  return data
}

export async function upgrade(id: string, version: string): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>(`/admin/modules/${encodeURIComponent(id)}/upgrade`, { version })
  return data
}

export async function rollback(id: string, version: string): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>(`/admin/modules/${encodeURIComponent(id)}/rollback`, { version })
  return data
}

export async function enable(id: string): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>(`/admin/modules/${encodeURIComponent(id)}/enable`)
  return data
}

export async function disable(id: string): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>(`/admin/modules/${encodeURIComponent(id)}/disable`)
  return data
}

export async function uninstall(id: string): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>(`/admin/modules/${encodeURIComponent(id)}/uninstall`)
  return data
}

export async function purge(id: string, confirm = true): Promise<InstalledModule> {
  const { data } = await apiClient.post<InstalledModule>(`/admin/modules/${encodeURIComponent(id)}/purge`, { confirm })
  return data
}

export default {
  listInstalled,
  listProviderAdapters,
  listProviderAccountForms,
  listMarketplace,
  install,
  installArchive,
  installFromMarketplace,
  permissions,
  approvePermissions,
  upgrade,
  rollback,
  enable,
  disable,
  uninstall,
  purge
}
