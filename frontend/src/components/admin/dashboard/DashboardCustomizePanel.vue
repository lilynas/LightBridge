<template>
  <BaseDialog
    :show="show"
    :title="t('admin.dashboard.customize.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.dashboard.customize.smallTitle') }}
          </h4>
          <span class="rounded-lg bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">
            {{ t('admin.dashboard.customize.smallCount', { count: enabledSmallPanels.length, limit: smallLimit }) }}
          </span>
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
          <div class="space-y-2">
            <div class="text-xs font-semibold uppercase text-gray-400 dark:text-gray-500">
              {{ t('admin.dashboard.customize.enabled') }}
            </div>
            <div
              v-if="enabledSmallPanels.length === 0"
              class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
            >
              {{ t('admin.dashboard.customize.emptyEnabled') }}
            </div>
            <div
              v-for="(panel, index) in enabledSmallPanels"
              :key="panel.key"
              class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700"
            >
              <span class="w-6 text-xs font-medium text-gray-400">{{ index + 1 }}</span>
              <Icon name="grid" size="sm" class="text-gray-400" />
              <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
                {{ labelOf(panel) }}
              </span>
              <button
                type="button"
                class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :title="t('admin.dashboard.customize.moveUp')"
                :aria-label="t('admin.dashboard.customize.moveUp')"
                :disabled="index === 0"
                @click="moveSmall(panel.key, -1)"
              >
                <Icon name="chevronUp" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :title="t('admin.dashboard.customize.moveDown')"
                :aria-label="t('admin.dashboard.customize.moveDown')"
                :disabled="index === enabledSmallPanels.length - 1"
                @click="moveSmall(panel.key, 1)"
              >
                <Icon name="chevronDown" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-300"
                :title="t('admin.dashboard.customize.remove')"
                :aria-label="t('admin.dashboard.customize.remove')"
                @click="removeSmall(panel.key)"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>

          <div class="space-y-2">
            <div class="text-xs font-semibold uppercase text-gray-400 dark:text-gray-500">
              {{ t('admin.dashboard.customize.hidden') }}
            </div>
            <div
              v-if="hiddenSmallPanels.length === 0"
              class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
            >
              {{ t('admin.dashboard.customize.emptyHidden') }}
            </div>
            <div
              v-for="panel in hiddenSmallPanels"
              :key="panel.key"
              class="flex items-center gap-2 rounded-lg border border-dashed border-gray-200 px-3 py-2 dark:border-dark-700"
              :class="{ 'opacity-60': smallLimitReached }"
            >
              <Icon name="grid" size="sm" class="text-gray-400" />
              <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
                {{ labelOf(panel) }}
              </span>
              <button
                type="button"
                class="rounded p-1 text-primary-600 transition-colors hover:bg-primary-50 disabled:text-gray-400 disabled:hover:bg-transparent dark:text-primary-400 dark:hover:bg-primary-900/20"
                :title="smallLimitReached ? t('admin.dashboard.customize.smallLimit', { limit: smallLimit }) : t('admin.dashboard.customize.add')"
                :aria-label="t('admin.dashboard.customize.add')"
                :disabled="smallLimitReached"
                @click="addSmall(panel.key)"
              >
                <Icon name="plus" size="sm" />
              </button>
            </div>
          </div>
        </div>
      </section>

      <section class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.dashboard.customize.largeTitle') }}
          </h4>
          <span class="rounded-lg bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">
            {{ enabledLargePanels.length }}
          </span>
        </div>

        <div class="grid gap-4 lg:grid-cols-2">
          <div class="space-y-2">
            <div class="text-xs font-semibold uppercase text-gray-400 dark:text-gray-500">
              {{ t('admin.dashboard.customize.enabled') }}
            </div>
            <div
              v-if="enabledLargePanels.length === 0"
              class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
            >
              {{ t('admin.dashboard.customize.emptyEnabled') }}
            </div>
            <div
              v-for="(panel, index) in enabledLargePanels"
              :key="panel.key"
              class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-700"
            >
              <span class="w-6 text-xs font-medium text-gray-400">{{ index + 1 }}</span>
              <Icon name="chart" size="sm" class="text-gray-400" />
              <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
                {{ labelOf(panel) }}
              </span>
              <button
                type="button"
                class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :title="t('admin.dashboard.customize.moveUp')"
                :aria-label="t('admin.dashboard.customize.moveUp')"
                :disabled="index === 0"
                @click="moveLarge(panel.key, -1)"
              >
                <Icon name="chevronUp" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                :title="t('admin.dashboard.customize.moveDown')"
                :aria-label="t('admin.dashboard.customize.moveDown')"
                :disabled="index === enabledLargePanels.length - 1"
                @click="moveLarge(panel.key, 1)"
              >
                <Icon name="chevronDown" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-1 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-300"
                :title="t('admin.dashboard.customize.remove')"
                :aria-label="t('admin.dashboard.customize.remove')"
                @click="removeLarge(panel.key)"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>

          <div class="space-y-2">
            <div class="text-xs font-semibold uppercase text-gray-400 dark:text-gray-500">
              {{ t('admin.dashboard.customize.hidden') }}
            </div>
            <div
              v-if="hiddenLargePanels.length === 0"
              class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
            >
              {{ t('admin.dashboard.customize.emptyHidden') }}
            </div>
            <div
              v-for="panel in hiddenLargePanels"
              :key="panel.key"
              class="flex items-center gap-2 rounded-lg border border-dashed border-gray-200 px-3 py-2 dark:border-dark-700"
            >
              <Icon name="chart" size="sm" class="text-gray-400" />
              <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
                {{ labelOf(panel) }}
              </span>
              <button
                type="button"
                class="rounded p-1 text-primary-600 transition-colors hover:bg-primary-50 dark:text-primary-400 dark:hover:bg-primary-900/20"
                :title="t('admin.dashboard.customize.add')"
                :aria-label="t('admin.dashboard.customize.add')"
                @click="addLarge(panel.key)"
              >
                <Icon name="plus" size="sm" />
              </button>
            </div>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('reset')">
          {{ t('admin.dashboard.customize.reset') }}
        </button>
        <button type="button" class="btn btn-primary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

interface DashboardPanelOption {
  key: string
  labelKey: string
}

const props = defineProps<{
  show: boolean
  smallPanels: DashboardPanelOption[]
  largePanels: DashboardPanelOption[]
  enabledSmallKeys: string[]
  enabledLargeKeys: string[]
  smallLimit: number
}>()

const emit = defineEmits<{
  close: []
  reset: []
  'update:enabledSmallKeys': [value: string[]]
  'update:enabledLargeKeys': [value: string[]]
}>()

const { t } = useI18n()

const enabledSmallPanelSet = computed(() => new Set(props.enabledSmallKeys))
const enabledLargePanelSet = computed(() => new Set(props.enabledLargeKeys))
const smallLimitReached = computed(() => props.enabledSmallKeys.length >= props.smallLimit)

const enabledSmallPanels = computed(() => panelsForKeys(props.enabledSmallKeys, props.smallPanels))
const enabledLargePanels = computed(() => panelsForKeys(props.enabledLargeKeys, props.largePanels))
const hiddenSmallPanels = computed(() => props.smallPanels.filter((panel) => !enabledSmallPanelSet.value.has(panel.key)))
const hiddenLargePanels = computed(() => props.largePanels.filter((panel) => !enabledLargePanelSet.value.has(panel.key)))

function panelsForKeys(keys: string[], panels: DashboardPanelOption[]): DashboardPanelOption[] {
  const panelMap = new Map(panels.map((panel) => [panel.key, panel]))
  return keys
    .map((key) => panelMap.get(key))
    .filter((panel): panel is DashboardPanelOption => Boolean(panel))
}

function labelOf(panel: DashboardPanelOption): string {
  return t(panel.labelKey)
}

function addSmall(key: string) {
  if (enabledSmallPanelSet.value.has(key) || smallLimitReached.value) return
  emit('update:enabledSmallKeys', [...props.enabledSmallKeys, key])
}

function removeSmall(key: string) {
  emit('update:enabledSmallKeys', props.enabledSmallKeys.filter((item) => item !== key))
}

function moveSmall(key: string, delta: number) {
  emit('update:enabledSmallKeys', moveKey(props.enabledSmallKeys, key, delta))
}

function addLarge(key: string) {
  if (enabledLargePanelSet.value.has(key)) return
  emit('update:enabledLargeKeys', [...props.enabledLargeKeys, key])
}

function removeLarge(key: string) {
  emit('update:enabledLargeKeys', props.enabledLargeKeys.filter((item) => item !== key))
}

function moveLarge(key: string, delta: number) {
  emit('update:enabledLargeKeys', moveKey(props.enabledLargeKeys, key, delta))
}

function moveKey(keys: string[], key: string, delta: number): string[] {
  const index = keys.indexOf(key)
  const nextIndex = index + delta
  if (index < 0 || nextIndex < 0 || nextIndex >= keys.length) {
    return keys
  }
  const next = [...keys]
  const [item] = next.splice(index, 1)
  next.splice(nextIndex, 0, item)
  return next
}
</script>
