<template>
  <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
    <div v-if="loading" class="space-y-3">
      <div class="h-4 w-40 animate-pulse rounded bg-gray-200 dark:bg-dark-700"></div>
      <div class="h-20 animate-pulse rounded bg-gray-100 dark:bg-dark-800"></div>
    </div>
    <div v-else-if="errorMessage" class="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
      <p>{{ errorMessage }}</p>
      <dl class="mt-3 grid gap-2 text-xs text-red-700/80 dark:text-red-200/80 sm:grid-cols-2">
        <div>
          <dt class="font-medium">Provider</dt>
          <dd class="break-all">{{ providerId }}</dd>
        </div>
        <div>
          <dt class="font-medium">Module</dt>
          <dd class="break-all">{{ moduleId || providerId }}</dd>
        </div>
        <div>
          <dt class="font-medium">Version</dt>
          <dd class="break-all">{{ moduleVersion || '-' }}</dd>
        </div>
        <div class="sm:col-span-2">
          <dt class="font-medium">Contribution</dt>
          <dd class="break-all">{{ exposedModule }}</dd>
        </div>
      </dl>
    </div>
    <component
      v-else-if="remoteComponent"
      :is="remoteComponent"
      :provider-id="providerId"
      :module-id="moduleId"
      @submit="$emit('submit', $event)"
      @cancel="$emit('cancel')"
    />
  </div>
</template>

<script setup lang="ts">
import { markRaw, ref, watch } from 'vue'
import type { Component } from 'vue'
import { versionedRemoteEntry } from '@/utils/modules/remoteEntry'

const props = defineProps<{
  providerId: string
  moduleId?: string
  moduleVersion?: string
  remoteEntry: string
  exposedModule: string
}>()

defineEmits<{
  submit: [payload: unknown]
  cancel: []
}>()

const loading = ref(false)
const errorMessage = ref('')
const remoteComponent = ref<Component | null>(null)

watch(
  () => [props.remoteEntry, props.exposedModule, props.moduleVersion],
  () => {
    void loadRemoteComponent()
  },
  { immediate: true }
)

async function loadRemoteComponent() {
  loading.value = true
  errorMessage.value = ''
  remoteComponent.value = null
  try {
    const remote = await import(/* @vite-ignore */ versionedRemoteEntry(props.remoteEntry, props.moduleVersion))
    const loaded = resolveRemoteComponent(remote, props.exposedModule)
    if (!loaded) {
      throw new Error(`Remote module ${props.exposedModule} was not exported`)
    }
    remoteComponent.value = markRaw(loaded)
  } catch (error) {
    console.error('Module account form remote failed', {
      providerId: props.providerId,
      moduleId: props.moduleId || props.providerId,
      moduleVersion: props.moduleVersion,
      remoteEntry: props.remoteEntry,
      exposedModule: props.exposedModule,
      error
    })
    errorMessage.value = error instanceof Error ? error.message : 'Failed to load module account form'
  } finally {
    loading.value = false
  }
}

function resolveRemoteComponent(remote: Record<string, unknown>, exposedModule: string): Component | null {
  const normalized = exposedModule.replace(/^\.\//, '')
  const candidates = [
    exposedModule,
    normalized,
    normalized.replace(/[^A-Za-z0-9_$]/g, ''),
    'default'
  ]
  for (const key of candidates) {
    const candidate = remote[key]
    if (candidate) return candidate as Component
  }
  return null
}
</script>
