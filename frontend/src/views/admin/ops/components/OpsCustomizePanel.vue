<template>
  <BaseDialog :show="show" :title="t('admin.ops.customize.title')" width="normal" @close="$emit('close')">
    <div class="space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.ops.customize.hint') }}
      </p>

      <div class="space-y-2">
        <!-- 已启用（按顺序） -->
        <div class="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
          {{ t('admin.ops.customize.enabled') }}
        </div>
        <div
          v-for="key in enabled"
          :key="key"
          class="flex items-center gap-3 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700"
        >
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :checked="isEnabled(key)"
            @change="toggle(key)"
          />
          <span class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ labelOf(key) }}</span>
          <div class="flex items-center gap-1">
            <button
              type="button"
              class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
              :disabled="enabled.indexOf(key) === 0"
              @click="moveUp(key)"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" /></svg>
            </button>
            <button
              type="button"
              class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
              :disabled="enabled.indexOf(key) === enabled.length - 1"
              @click="moveDown(key)"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" /></svg>
            </button>
          </div>
        </div>

        <!-- 未启用 -->
        <div class="mb-1 mt-4 text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
          {{ t('admin.ops.customize.disabled') }}
        </div>
        <div
          v-for="card in allCards.filter((c) => !isEnabled(c.key))"
          :key="card.key"
          class="flex items-center gap-3 rounded-lg border border-dashed border-gray-200 px-3 py-2 opacity-70 dark:border-dark-600"
        >
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :checked="false"
            @change="toggle(card.key)"
          />
          <span class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ labelOf(card.key) }}</span>
        </div>
        <p v-if="!allCards.some((c) => !isEnabled(c.key))" class="text-xs text-gray-400">
          {{ t('admin.ops.customize.allEnabled') }}
        </p>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="reset">
          {{ t('common.reset') }}
        </button>
        <button type="button" class="btn btn-primary" @click="$emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useOpsConsoleLayout } from '@/composables/useOpsConsoleLayout'

defineProps<{ show: boolean }>()
defineEmits<{ (e: 'close'): void }>()

const { t, te } = useI18n()
const { enabled, allCards, isEnabled, toggle, moveUp, moveDown, reset } = useOpsConsoleLayout()

function labelOf(key: string): string {
  const def = allCards.find((c) => c.key === key)
  if (!def) return key
  const k = def.label
  return te(k) ? t(k) : k
}
</script>
