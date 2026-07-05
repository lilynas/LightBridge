<template>
  <Teleport to="body">
    <transition name="fade">
      <div
        v-if="visible"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        @click.self="$emit('close')"
      >
        <div class="w-full max-w-md rounded-xl bg-white shadow-xl dark:bg-dark-800" @click.stop>
          <div class="border-b border-gray-200 px-6 py-4 dark:border-dark-700">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('modelCatalog.quickMonitorTitle', { model: modelId }) }}
            </h3>
          </div>

          <form class="px-6 py-4 space-y-4" @submit.prevent="handleSubmit">
            <!-- Provider -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('modelCatalog.quickMonitorProvider') }}
              </label>
              <select
                v-model="form.provider"
                class="input w-full"
                required
              >
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
              </select>
            </div>

            <!-- Endpoint -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('modelCatalog.quickMonitorEndpoint') }}
              </label>
              <input
                v-model="form.endpoint"
                type="url"
                class="input w-full"
                placeholder="https://api.openai.com"
                required
              />
            </div>

            <!-- API Key -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                API Key
              </label>
              <input
                v-model="form.api_key"
                type="password"
                class="input w-full"
                placeholder="sk-..."
                required
              />
            </div>

            <!-- API Mode -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('modelCatalog.quickMonitorApiMode') }}
              </label>
              <select v-model="form.api_mode" class="input w-full">
                <option value="chat_completions">Chat Completions</option>
                <option value="responses">Responses</option>
              </select>
            </div>

            <!-- Interval -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('modelCatalog.quickMonitorInterval') }}
              </label>
              <select v-model.number="form.interval_seconds" class="input w-full">
                <option :value="30">30s</option>
                <option :value="60">60s</option>
                <option :value="120">120s</option>
                <option :value="300">300s</option>
              </select>
            </div>

            <!-- Error -->
            <div v-if="error" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
              {{ error }}
            </div>

            <!-- Actions -->
            <div class="flex justify-end gap-3 pt-2">
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="submitting"
                @click="$emit('close')"
              >
                {{ t('common.cancel', 'Cancel') }}
              </button>
              <button
                type="submit"
                class="btn btn-primary"
                :disabled="submitting"
              >
                <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
                {{ t('modelCatalog.quickMonitorSubmit') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { createQuickMonitor } from '@/api/admin/quickMonitor'

const props = defineProps<{
  visible: boolean
  modelId: string
  platform?: string
}>()

const emit = defineEmits<{
  close: []
  created: []
}>()

const { t } = useI18n()
const submitting = ref(false)
const error = ref('')

function inferProvider(platform?: string): string {
  const p = (platform || '').toLowerCase()
  if (p.includes('openai') || p.includes('gpt')) return 'openai'
  if (p.includes('anthropic') || p.includes('claude')) return 'anthropic'
  if (p.includes('gemini') || p.includes('google')) return 'gemini'
  return 'openai'
}

const form = reactive({
  provider: inferProvider(props.platform),
  endpoint: '',
  api_key: '',
  api_mode: 'chat_completions',
  interval_seconds: 60,
})

async function handleSubmit() {
  submitting.value = true
  error.value = ''
  try {
    await createQuickMonitor({
      model_id: props.modelId,
      provider: form.provider,
      api_mode: form.api_mode,
      endpoint: form.endpoint,
      api_key: form.api_key,
      interval_seconds: form.interval_seconds,
    })
    emit('created')
    emit('close')
  } catch (e: any) {
    error.value = e?.response?.data?.message || e?.message || 'Failed to create monitor'
  } finally {
    submitting.value = false
  }
}
</script>
