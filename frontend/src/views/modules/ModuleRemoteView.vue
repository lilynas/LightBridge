<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
        {{ routeTitle || moduleName }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ moduleName }}
      </p>
    </div>

    <div v-if="loading" class="card p-6">
      <div class="h-5 w-48 animate-pulse rounded bg-gray-200 dark:bg-dark-700"></div>
      <div class="mt-4 h-24 animate-pulse rounded bg-gray-100 dark:bg-dark-800"></div>
    </div>

    <div v-else-if="errorMessage" class="card border-red-200 p-6 dark:border-red-900/60">
      <h2 class="text-base font-semibold text-red-700 dark:text-red-300">
        Module UI failed to load
      </h2>
      <p class="mt-2 text-sm text-red-600 dark:text-red-300">
        {{ errorMessage }}
      </p>
      <dl class="mt-4 grid gap-2 text-xs text-gray-600 dark:text-gray-300 sm:grid-cols-2">
        <div>
          <dt class="font-medium text-gray-800 dark:text-gray-100">Module ID</dt>
          <dd class="break-all">{{ moduleId }}</dd>
        </div>
        <div>
          <dt class="font-medium text-gray-800 dark:text-gray-100">Version</dt>
          <dd class="break-all">{{ moduleVersion || '-' }}</dd>
        </div>
        <div>
          <dt class="font-medium text-gray-800 dark:text-gray-100">Route</dt>
          <dd class="break-all">{{ routeTitle }}</dd>
        </div>
        <div>
          <dt class="font-medium text-gray-800 dark:text-gray-100">Contribution</dt>
          <dd class="break-all">{{ exposedModule }}</dd>
        </div>
      </dl>
    </div>

    <component
      :is="remoteComponent"
      v-else-if="remoteComponent"
      :module-id="moduleId"
      :module-name="moduleName"
    />
  </div>
</template>

<script setup lang="ts">
import { markRaw, onMounted, shallowRef, type Component } from 'vue'
import { versionedRemoteEntry } from '@/utils/modules/remoteEntry'

const props = defineProps<{
  moduleId: string
  moduleName: string
  moduleVersion?: string
  routeTitle: string
  remoteEntry: string
  exposedModule: string
}>()

const loading = shallowRef(true)
const errorMessage = shallowRef('')
const remoteComponent = shallowRef<Component | null>(null)

onMounted(async () => {
  try {
    const mod = await import(/* @vite-ignore */ versionedRemoteEntry(props.remoteEntry, props.moduleVersion))
    const component = resolveRemoteComponent(mod, props.exposedModule)
    if (!component) {
      throw new Error(`exposed module ${props.exposedModule} was not found`)
    }
    remoteComponent.value = markRaw(component)
  } catch (error) {
    console.error('Module UI remote failed', {
      moduleId: props.moduleId,
      moduleVersion: props.moduleVersion,
      routeTitle: props.routeTitle,
      remoteEntry: props.remoteEntry,
      exposedModule: props.exposedModule,
      error
    })
    errorMessage.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
})

function resolveRemoteComponent(mod: Record<string, unknown>, exposedModule: string): Component | null {
  const normalized = exposedModule.replace(/^\.\//, '')
  const candidates = [
    exposedModule,
    normalized,
    normalized.replace(/[^A-Za-z0-9_$]/g, ''),
    'default'
  ]
  for (const key of candidates) {
    const candidate = mod[key]
    if (candidate) return candidate as Component
  }
  return null
}
</script>
