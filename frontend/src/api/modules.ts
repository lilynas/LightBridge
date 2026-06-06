import { apiClient } from './client'

export interface ModuleUIRoute {
  path: string
  title: string
  remoteEntry: string
  exposedModule: string
  requiresAdmin?: boolean
}

export interface ModuleUIMenu {
  title: string
  path: string
  group?: string
  order?: number
}

export interface ModuleUIAccountForm {
  providerId: string
  providerName?: string
  moduleId?: string
  moduleName?: string
  moduleVersion?: string
  remoteEntry: string
  exposedModule: string
}

export interface ModuleUIManifestItem {
  moduleId: string
  moduleName: string
  version: string
  remoteEntry: string
  routes?: ModuleUIRoute[]
  menu?: ModuleUIMenu[]
  accountForms?: ModuleUIAccountForm[]
}

export async function getUIManifest(): Promise<ModuleUIManifestItem[]> {
  const { data } = await apiClient.get<ModuleUIManifestItem[]>('/modules/ui-manifest')
  return data
}

export default {
  getUIManifest
}
