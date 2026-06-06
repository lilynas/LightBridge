import type { Router, RouteRecordRaw } from 'vue-router'
import { getUIManifest, type ModuleUIManifestItem, type ModuleUIRoute } from '@/api/modules'

const moduleRouteNames = new Set<string>()
const moduleRoutePaths = new Set<string>()
let loaded = false
let loading: Promise<void> | null = null

export function resetModuleRouteCacheForTests(): void {
  moduleRouteNames.clear()
  moduleRoutePaths.clear()
  loaded = false
  loading = null
}

export async function ensureModuleRoutesRegistered(router: Router): Promise<void> {
  if (loaded) return
  if (loading) return loading

  loading = registerModuleRoutes(router)
    .finally(() => {
      loading = null
    })

  return loading
}

async function registerModuleRoutes(router: Router): Promise<void> {
  const manifests = await getUIManifest()
  for (const manifest of manifests) {
    const routes = Array.isArray(manifest.routes) ? manifest.routes : []
    for (const route of routes) {
      const record = toRouteRecord(manifest, route)
      if (
        !record ||
        moduleRouteNames.has(String(record.name)) ||
        moduleRoutePaths.has(record.path) ||
        router.hasRoute(String(record.name)) ||
        router.getRoutes().some((existing) => existing.path === record.path)
      ) {
        continue
      }
      router.addRoute(record)
      moduleRouteNames.add(String(record.name))
      moduleRoutePaths.add(record.path)
    }
  }
  loaded = true
}

function isValidRouteContribution(manifest: ModuleUIManifestItem, route: ModuleUIRoute): boolean {
  return typeof manifest.moduleId === 'string' &&
    manifest.moduleId.length > 0 &&
    typeof manifest.moduleName === 'string' &&
    manifest.moduleName.length > 0 &&
    typeof manifest.version === 'string' &&
    typeof route.path === 'string' &&
    route.path.startsWith('/admin/') &&
    typeof route.title === 'string' &&
    route.title.length > 0 &&
    typeof route.remoteEntry === 'string' &&
    route.remoteEntry.length > 0 &&
    typeof route.exposedModule === 'string' &&
    route.exposedModule.startsWith('./')
}

function toRouteRecord(manifest: ModuleUIManifestItem, route: ModuleUIRoute): RouteRecordRaw | null {
  if (!isValidRouteContribution(manifest, route)) {
    return null
  }
  const name = `Module:${manifest.moduleId}:${route.path}`
  return {
    path: route.path,
    name,
    component: () => import('@/views/modules/ModuleRemoteView.vue'),
    props: {
      moduleId: manifest.moduleId,
      moduleName: manifest.moduleName,
      moduleVersion: manifest.version,
      routeTitle: route.title,
      remoteEntry: route.remoteEntry,
      exposedModule: route.exposedModule
    },
    meta: {
      requiresAuth: true,
      requiresAdmin: route.requiresAdmin !== false,
      title: route.title
    }
  }
}
