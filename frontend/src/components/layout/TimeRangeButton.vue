<template>
  <div class="relative" ref="wrapperRef">
    <button
      type="button"
      class="btn-ghost btn-icon"
      :title="t('admin.dashboard.timeRange')"
      @click="open = !open"
    >
      <Icon name="clock" size="md" :stroke-width="2" />
    </button>

    <BaseDialog
      :show="open"
      :title="t('admin.dashboard.timeRange')"
      width="narrow"
      @close="open = false"
    >
      <div class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.dashboard.timeRange') }}</label>
          <DateRangePicker
            :start-date="draftStart"
            :end-date="draftEnd"
            @update:start-date="draftStart = $event"
            @update:end-date="draftEnd = $event"
            @change="onPickerChange"
          />
        </div>

        <div>
          <label class="input-label">{{ t('admin.dashboard.granularity') }}</label>
          <div class="mt-2 grid grid-cols-2 gap-2">
            <button
              v-for="opt in granularityOptions"
              :key="opt.value"
              type="button"
              @click="draftGranularity = opt.value"
              :class="[
                'rounded-lg border-2 px-3 py-2 text-sm font-medium transition-all',
                draftGranularity === opt.value
                  ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                  : 'border-gray-200 text-gray-600 hover:border-primary-300 dark:border-dark-600 dark:text-dark-300'
              ]"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-between gap-3">
          <button type="button" class="btn btn-secondary" @click="handleReset">
            {{ t('common.reset') }}
          </button>
          <div class="flex gap-3">
            <button type="button" class="btn btn-secondary" @click="open = false">
              {{ t('common.close') }}
            </button>
            <button type="button" class="btn btn-primary" @click="handleApply">
              {{ t('common.apply') }}
            </button>
          </div>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import { useTimeRangeStore, type DashboardGranularity } from '@/stores/timeRange'

const { t } = useI18n()
const store = useTimeRangeStore()

const open = ref(false)
const wrapperRef = ref<HTMLElement | null>(null)
const draftStart = ref(store.startDate)
const draftEnd = ref(store.endDate)
const draftGranularity = ref<DashboardGranularity>(store.granularity)

watch(open, (val) => {
  if (val) {
    draftStart.value = store.startDate
    draftEnd.value = store.endDate
    draftGranularity.value = store.granularity
  }
})

const granularityOptions = [
  { value: 'hour' as const, label: t('admin.dashboard.hour') },
  { value: 'day' as const, label: t('admin.dashboard.day') }
]

function onPickerChange(range: { startDate: string; endDate: string }) {
  if (range.startDate) draftStart.value = range.startDate
  if (range.endDate) draftEnd.value = range.endDate
}

function handleApply() {
  store.setRange(draftStart.value, draftEnd.value)
  store.setGranularity(draftGranularity.value)
  open.value = false
}

function handleReset() {
  store.reset()
  draftStart.value = store.startDate
  draftEnd.value = store.endDate
  draftGranularity.value = store.granularity
}
</script>
