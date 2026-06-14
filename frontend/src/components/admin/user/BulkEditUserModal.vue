<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.bulkEditTitle', { count: userIds.length })"
    width="normal"
    @close="$emit('close')"
  >
    <div class="space-y-5">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.users.bulkEditHint') }}
      </p>

      <!-- 状态 -->
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <label class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
          <input
            v-model="form.update_status"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          {{ t('admin.users.columns.status') }}
        </label>
        <div v-if="form.update_status" class="mt-3 grid grid-cols-2 gap-2">
          <button
            v-for="opt in statusOptions"
            :key="opt.value"
            type="button"
            @click="form.status = opt.value"
            :class="[
              'rounded-lg border-2 px-3 py-2 text-sm font-medium transition-all',
              form.status === opt.value
                ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                : 'border-gray-200 text-gray-600 dark:border-dark-600 dark:text-dark-300'
            ]"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <!-- 并发数 -->
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <label class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
          <input
            v-model="form.update_concurrency"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          {{ t('admin.users.columns.concurrency') }}
        </label>
        <input
          v-if="form.update_concurrency"
          v-model.number="form.concurrency"
          type="number"
          min="0"
          class="input mt-3"
          :placeholder="t('admin.users.bulkEditConcurrencyPlaceholder')"
        />
      </div>

      <!-- 备注 -->
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <label class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
          <input
            v-model="form.update_notes"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          {{ t('admin.users.columns.notes') }}
        </label>
        <textarea
          v-if="form.update_notes"
          v-model="form.notes"
          rows="3"
          class="input mt-3"
          :placeholder="t('admin.users.bulkEditNotesPlaceholder')"
        ></textarea>
      </div>

      <!-- 分组 -->
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <label class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
          <input
            v-model="form.update_groups"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          {{ t('admin.users.columns.groups') }}
        </label>
        <div v-if="form.update_groups" class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
          <label
            v-for="group in groups"
            :key="group.id"
            class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-600"
          >
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="form.group_ids.includes(group.id)"
              @change="toggleGroup(group.id)"
            />
            <span class="truncate text-gray-700 dark:text-gray-300">{{ group.name }}</span>
          </label>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!canSubmit || submitting"
          @click="handleSubmit"
        >
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { AdminGroup } from '@/types'
import type { BatchUpdateUsersRequest } from '@/api/admin/users'

const props = defineProps<{
  show: boolean
  userIds: number[]
  groups: AdminGroup[]
  submitting?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: BatchUpdateUsersRequest): void
}>()

const { t } = useI18n()

const form = reactive<{
  update_status: boolean
  status: 'active' | 'inactive'
  update_concurrency: boolean
  concurrency: number | null
  update_notes: boolean
  notes: string
  update_groups: boolean
  group_ids: number[]
}>({
  update_status: false,
  status: 'active',
  update_concurrency: false,
  concurrency: null,
  update_notes: false,
  notes: '',
  update_groups: false,
  group_ids: [],
})

const statusOptions = computed(() => [
  { value: 'active' as const, label: t('admin.users.statusActive') },
  { value: 'inactive' as const, label: t('admin.users.statusInactive') },
])

const canSubmit = computed(() => {
  if (form.update_status) return true
  if (form.update_concurrency && form.concurrency !== null) return true
  if (form.update_notes) return true
  if (form.update_groups) return true
  return false
})

function toggleGroup(id: number) {
  const idx = form.group_ids.indexOf(id)
  if (idx >= 0) form.group_ids.splice(idx, 1)
  else form.group_ids.push(id)
}

function handleSubmit() {
  if (!canSubmit.value) return
  const payload: BatchUpdateUsersRequest = { user_ids: [...props.userIds] }
  if (form.update_status) {
    payload.update_status = true
    payload.status = form.status
  }
  if (form.update_concurrency && form.concurrency !== null) {
    payload.update_concurrency = true
    payload.concurrency = form.concurrency
  }
  if (form.update_notes) {
    payload.update_notes = true
    payload.notes = form.notes
  }
  if (form.update_groups) {
    payload.update_groups = true
    payload.group_ids = [...form.group_ids]
  }
  emit('submit', payload)
}

// 每次打开重置
watch(
  () => props.show,
  (val) => {
    if (val) {
      form.update_status = false
      form.status = 'active'
      form.update_concurrency = false
      form.concurrency = null
      form.update_notes = false
      form.notes = ''
      form.update_groups = false
      form.group_ids = []
    }
  }
)
</script>
