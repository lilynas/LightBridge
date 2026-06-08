<template>
  <div class="space-y-6">
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.uiThemes.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.uiThemes.description') }}
        </p>
      </div>
      <div class="grid gap-4 p-6 lg:grid-cols-2">
        <div class="space-y-3">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.uiThemes.githubUrl') }}
          </label>
          <div class="flex gap-2">
            <input v-model.trim="githubUrl" type="url" class="input flex-1" placeholder="https://github.com/org/theme" />
            <button type="button" class="btn btn-primary" :disabled="busy || !githubUrl" @click="importFromGitHub">
              <Icon name="download" size="sm" class="mr-1.5" />
              {{ t('admin.settings.uiThemes.import') }}
            </button>
          </div>
        </div>
        <div class="space-y-3">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.uiThemes.zipUpload') }}
          </label>
          <div class="flex gap-2">
            <input ref="fileInput" type="file" accept=".zip" class="input flex-1" @change="onFileChange" />
            <button type="button" class="btn btn-secondary" :disabled="busy || !selectedFile" @click="uploadSelected">
              <Icon name="upload" size="sm" class="mr-1.5" />
              {{ t('admin.settings.uiThemes.upload') }}
            </button>
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <input v-model="replaceExisting" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
          {{ t('admin.settings.uiThemes.replaceExisting') }}
        </label>
      </div>
    </div>

    <div v-if="error" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="card overflow-hidden">
      <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.uiThemes.installed') }}
        </h3>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="loadThemes">
          <Icon name="refresh" size="sm" class="mr-1.5" />
          {{ t('common.refresh') }}
        </button>
      </div>
      <div v-if="loading" class="flex justify-center py-10">
        <div class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <div v-else-if="themes.length === 0" class="px-6 py-10 text-center text-sm text-gray-500">
        {{ t('admin.settings.uiThemes.empty') }}
      </div>
      <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
        <div v-for="theme in themes" :key="theme.id" class="grid gap-4 px-6 py-5 lg:grid-cols-[1fr_auto]">
          <div class="min-w-0 space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <h4 class="font-semibold text-gray-900 dark:text-white">{{ theme.name }}</h4>
              <code class="rounded bg-gray-100 px-2 py-0.5 text-xs dark:bg-dark-700">{{ theme.id }}</code>
              <span v-if="theme.active" class="rounded bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-300">
                {{ t('admin.settings.uiThemes.active') }}
              </span>
            </div>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              v{{ theme.version }} · {{ theme.source || '-' }}
            </p>
            <textarea
              v-model="configDrafts[theme.id]"
              class="input min-h-[120px] w-full font-mono text-xs"
              spellcheck="false"
            ></textarea>
          </div>
          <div class="flex flex-wrap content-start gap-2 lg:justify-end">
            <button type="button" class="btn btn-primary btn-sm" :disabled="busy || theme.active" @click="activate(theme.id)">
              {{ t('admin.settings.uiThemes.activate') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy || !theme.active" @click="deactivate(theme.id)">
              {{ t('admin.settings.uiThemes.deactivate') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="saveConfig(theme.id)">
              {{ t('common.save') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400" :disabled="busy" @click="remove(theme.id)">
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import uiThemesAPI, { type UITheme } from '@/api/admin/uiThemes'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const themes = ref<UITheme[]>([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const githubUrl = ref('')
const selectedFile = ref<File | null>(null)
const replaceExisting = ref(false)
const configDrafts = reactive<Record<string, string>>({})

function setThemes(items: UITheme[]) {
  themes.value = items
  for (const theme of items) {
    configDrafts[theme.id] = JSON.stringify(theme.config || {}, null, 2)
  }
}

async function loadThemes() {
  loading.value = true
  error.value = ''
  try {
    setThemes(await uiThemesAPI.listThemes())
  } catch (err) {
    error.value = extractApiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFile.value = input.files?.[0] ?? null
}

async function run(operation: () => Promise<unknown>) {
  busy.value = true
  error.value = ''
  try {
    await operation()
    await loadThemes()
  } catch (err) {
    error.value = extractApiErrorMessage(err)
  } finally {
    busy.value = false
  }
}

function importFromGitHub() {
  return run(() => uiThemesAPI.importGitHubTheme(githubUrl.value, replaceExisting.value))
}

function uploadSelected() {
  if (!selectedFile.value) return
  return run(() => uiThemesAPI.uploadTheme(selectedFile.value as File, replaceExisting.value))
}

function activate(id: string) {
  return run(() => uiThemesAPI.activateTheme(id))
}

function deactivate(id: string) {
  return run(() => uiThemesAPI.deactivateTheme(id))
}

function saveConfig(id: string) {
  return run(async () => {
    const parsed = JSON.parse(configDrafts[id] || '{}')
    await uiThemesAPI.updateThemeConfig(id, parsed)
  })
}

function remove(id: string) {
  return run(() => uiThemesAPI.deleteTheme(id))
}

onMounted(loadThemes)
</script>
