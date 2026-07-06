<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataExportFormat')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataExportFormatHint') }}
      </p>

      <div class="space-y-2">
        <label
          v-for="format in formatOptions"
          :key="format.value"
          class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-all"
          :class="selectedFormat === format.value
            ? 'border-primary-500 bg-primary-50 dark:border-primary-600 dark:bg-primary-900/20'
            : 'border-gray-200 hover:border-gray-300 dark:border-dark-600 dark:hover:border-dark-500'"
        >
          <input
            v-model="selectedFormat"
            type="radio"
            :value="format.value"
            class="mt-0.5 h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <div class="min-w-0 flex-1">
            <div class="text-sm font-medium text-gray-900 dark:text-white">
              {{ format.label }}
            </div>
            <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{ format.description }}
            </div>
          </div>
        </label>
      </div>

      <!-- Include proxies checkbox -->
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" v-model="includeProxies" />
        <span>{{ t('admin.accounts.dataExportIncludeProxies') }}</span>
      </label>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="exporting" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="button"
          :disabled="exporting"
          @click="handleExport"
        >
          {{ exporting ? t('admin.accounts.dataExporting') : t('admin.accounts.dataExportConfirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { OutputFormat } from '@/utils/authconv'
import type { AdminDataPayload } from '@/types'

type ExportFormat = OutputFormat | 'native'

interface Props {
  show: boolean
  exporting: boolean
  payload: AdminDataPayload | null
  accountCount: number
}

interface Emits {
  (e: 'close'): void
  (e: 'export', format: ExportFormat, includeProxies: boolean): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()

const selectedFormat = ref<ExportFormat>('native')
const includeProxies = ref(true)

interface FormatOption {
  value: ExportFormat
  label: string
  description: string
}

const formatOptions = computed<FormatOption[]>(() => [
  {
    value: 'native',
    label: t('admin.accounts.dataExportFormatNative'),
    description: t('admin.accounts.dataExportFormatNativeDesc')
  },
  {
    value: 'cpa',
    label: t('admin.accounts.dataExportFormatCpa'),
    description: t('admin.accounts.dataExportFormatCpaDesc')
  },
  {
    value: 'sub2api',
    label: t('admin.accounts.dataExportFormatSub2api'),
    description: t('admin.accounts.dataExportFormatSub2apiDesc')
  },
  {
    value: 'codex2api',
    label: t('admin.accounts.dataExportFormatCodex2api'),
    description: t('admin.accounts.dataExportFormatCodex2apiDesc')
  },
  {
    value: 'codexmanager',
    label: t('admin.accounts.dataExportFormatCodexManager'),
    description: t('admin.accounts.dataExportFormatCodexManagerDesc')
  },
  {
    value: 'codex',
    label: t('admin.accounts.dataExportFormatCodexAuth'),
    description: t('admin.accounts.dataExportFormatCodexAuthDesc')
  }
])

const handleClose = () => {
  if (props.exporting) return
  emit('close')
}

const handleExport = () => {
  emit('export', selectedFormat.value, includeProxies.value)
}
</script>
