<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">

      <div v-if="loading" class="card p-8 text-center text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <!-- 顶部保存栏 -->
        <div class="card flex items-center justify-end gap-3 px-6 py-3">
          <span v-if="statusMessage" :class="statusError ? 'text-red-600' : 'text-green-600'" class="text-sm">
            {{ statusMessage }}
          </span>
          <button class="btn btn-primary" type="button" :disabled="saving" @click="save">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>

        <!-- 基础开关 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyFilter.basic.title') }}
            </h2>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.privacyFilter.basic.enabled') }}
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyFilter.basic.enabledHint') }}
                </p>
              </div>
              <Toggle v-model="form.enabled" />
            </div>
            <div class="flex items-center justify-between">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.privacyFilter.basic.filterRequest') }}
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyFilter.basic.filterRequestHint') }}
                </p>
              </div>
              <Toggle v-model="form.filter_request" />
            </div>
            <div class="flex items-center justify-between">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.privacyFilter.basic.filterResponse') }}
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyFilter.basic.filterResponseHint') }}
                </p>
              </div>
              <Toggle v-model="form.filter_response" />
            </div>
          </div>
        </div>

        <!-- 内置规则 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyFilter.builtin.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.privacyFilter.builtin.description') }}
            </p>
          </div>
          <div class="grid grid-cols-1 gap-4 p-6 sm:grid-cols-2 lg:grid-cols-3">
            <label
              v-for="id in builtinIds"
              :key="id"
              class="flex items-center gap-3 rounded-lg border border-gray-200 px-4 py-3 transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
            >
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="form.builtin_rules[id] !== false"
                @change="toggleBuiltin(id, ($event.target as HTMLInputElement).checked)"
              />
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ builtinLabel(id) }}</span>
            </label>
          </div>
        </div>

        <!-- 自定义规则 -->
        <div class="card">
          <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.privacyFilter.custom.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.privacyFilter.custom.description') }}
              </p>
            </div>
            <button class="btn btn-secondary" type="button" @click="addCustomRule">
              {{ t('admin.privacyFilter.custom.add') }}
            </button>
          </div>
          <div class="space-y-3 p-6">
            <p v-if="form.custom_rules.length === 0" class="text-sm text-gray-400">
              {{ t('admin.privacyFilter.custom.empty') }}
            </p>
            <div
              v-for="(rule, index) in form.custom_rules"
              :key="index"
              class="grid grid-cols-1 items-center gap-3 rounded-lg border border-gray-200 p-4 dark:border-dark-700 md:grid-cols-12"
            >
              <input
                v-model="rule.name"
                class="input md:col-span-3"
                :placeholder="t('admin.privacyFilter.custom.namePlaceholder')"
              />
              <input
                v-model="rule.pattern"
                class="input font-mono md:col-span-4"
                :placeholder="t('admin.privacyFilter.custom.patternPlaceholder')"
              />
              <input
                v-model="rule.replacement"
                class="input md:col-span-3"
                :placeholder="t('admin.privacyFilter.custom.replacementPlaceholder')"
              />
              <div class="flex items-center justify-end gap-3 md:col-span-2">
                <Toggle v-model="rule.enabled" />
                <button
                  class="text-sm text-red-600 hover:underline dark:text-red-400"
                  type="button"
                  @click="removeCustomRule(index)"
                >
                  {{ t('common.delete') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 应用对象（针对谁过滤） -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyFilter.target.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.privacyFilter.target.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <button
                v-for="opt in targetScopeOptions"
                :key="opt.value"
                type="button"
                @click="form.target_scope = opt.value"
                :class="[
                  'rounded-lg border-2 px-4 py-3 text-left text-sm font-medium transition-all',
                  form.target_scope === opt.value
                    ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                    : 'border-gray-200 text-gray-700 hover:border-primary-300 dark:border-dark-700 dark:text-dark-200'
                ]"
              >
                {{ opt.label }}
              </button>
            </div>
            <div v-if="form.target_scope === 'partial_users'" class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.privacyFilter.target.selectUsers') }}
              </label>
              <input
                :value="targetUserSearch"
                @input="searchTargetUsers(($event.target as HTMLInputElement).value)"
                type="text"
                class="input"
                :placeholder="t('admin.privacyFilter.target.searchUsers')"
              />
              <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.privacyFilter.target.selectedCount', { count: form.target_user_ids.length }) }}
              </div>
              <div class="mt-2 grid max-h-48 grid-cols-1 gap-1 overflow-y-auto sm:grid-cols-2">
                <label
                  v-for="u in targetUserOptions"
                  :key="u.id"
                  class="flex items-center gap-2 rounded-md border border-gray-200 px-3 py-1.5 text-sm dark:border-dark-700"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="form.target_user_ids.includes(u.id)"
                    @change="toggleArrayId(form.target_user_ids, u.id)"
                  />
                  <span class="truncate text-gray-700 dark:text-gray-300">{{ u.label }}</span>
                </label>
                <p v-if="!targetUserOptions.length && !targetUserLoading" class="col-span-full text-center text-xs text-gray-400">
                  {{ t('admin.privacyFilter.target.noUsers') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- 渠道维度 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyFilter.channel.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.privacyFilter.channel.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-4">
              <button
                v-for="opt in channelScopeOptions"
                :key="opt.value"
                type="button"
                @click="form.channel_scope = opt.value"
                :class="[
                  'rounded-lg border-2 px-4 py-3 text-left text-sm font-medium transition-all',
                  form.channel_scope === opt.value
                    ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
                    : 'border-gray-200 text-gray-700 hover:border-primary-300 dark:border-dark-700 dark:text-dark-200'
                ]"
              >
                {{ opt.label }}
              </button>
            </div>
            <!-- 按分组 -->
            <div v-if="form.channel_scope === 'group'">
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.privacyFilter.channel.selectGroups') }}
              </label>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
                <label
                  v-for="group in groups"
                  :key="group.id"
                  class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="form.channel_ids.includes(group.id)"
                    @change="toggleArrayId(form.channel_ids, group.id)"
                  />
                  <span class="truncate text-sm text-gray-700 dark:text-gray-300">{{ group.name }}</span>
                </label>
              </div>
            </div>
            <!-- 按渠道 -->
            <div v-if="form.channel_scope === 'channel'">
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.privacyFilter.channel.selectChannels') }}
              </label>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
                <label
                  v-for="ch in channelOptions"
                  :key="ch.id"
                  class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="form.channel_ids.includes(ch.id)"
                    @change="toggleArrayId(form.channel_ids, ch.id)"
                  />
                  <span class="truncate text-sm text-gray-700 dark:text-gray-300">{{ ch.label }}</span>
                </label>
                <p v-if="!channelOptions.length" class="col-span-full text-center text-xs text-gray-400">
                  {{ t('admin.privacyFilter.channel.noChannels') }}
                </p>
              </div>
            </div>
            <!-- 按账号 -->
            <div v-if="form.channel_scope === 'account'">
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.privacyFilter.channel.selectAccounts') }}
              </label>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
                <label
                  v-for="acc in accountOptions"
                  :key="acc.id"
                  class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="form.account_ids.includes(acc.id)"
                    @change="toggleArrayId(form.account_ids, acc.id)"
                  />
                  <span class="truncate text-sm text-gray-700 dark:text-gray-300">{{ acc.label }}</span>
                </label>
                <p v-if="!accountOptions.length" class="col-span-full text-center text-xs text-gray-400">
                  {{ t('admin.privacyFilter.channel.noAccounts') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- 作用域 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.privacyFilter.scope.title') }}
            </h2>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.privacyFilter.scope.allGroups') }}
                </label>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.privacyFilter.scope.allGroupsHint') }}
                </p>
              </div>
              <Toggle v-model="form.all_groups" />
            </div>
            <div v-if="!form.all_groups">
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.privacyFilter.scope.groups') }}
              </label>
              <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
                <label
                  v-for="group in groups"
                  :key="group.id"
                  class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 transition-colors hover:border-primary-300 dark:border-dark-700 dark:hover:border-primary-700"
                >
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :checked="form.group_ids.includes(group.id)"
                    @change="toggleGroup(group.id, ($event.target as HTMLInputElement).checked)"
                  />
                  <span class="truncate text-sm text-gray-700 dark:text-gray-300">{{ group.name }}</span>
                </label>
              </div>
            </div>
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.privacyFilter.scope.modelFilter') }}
              </label>
              <Select v-model="form.model_filter.type" :options="modelFilterOptions" class="max-w-sm" />
              <textarea
                v-if="form.model_filter.type !== 'all'"
                v-model="modelsText"
                class="input mt-3 h-32 w-full font-mono"
                :placeholder="t('admin.privacyFilter.scope.modelsPlaceholder')"
              />
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type {
  PrivacyFilterConfig,
  PrivacyFilterModelFilterType,
  PrivacyFilterRule,
} from '@/api/admin/privacyFilter'
import type { AdminGroup, SelectOption } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()

const loading = ref(true)
const saving = ref(false)
const statusMessage = ref('')
const statusError = ref(false)
const groups = ref<AdminGroup[]>([])
const builtinIds = ref<string[]>([])

const form = reactive<{
  enabled: boolean
  filter_request: boolean
  filter_response: boolean
  builtin_rules: Record<string, boolean>
  custom_rules: PrivacyFilterRule[]
  all_groups: boolean
  group_ids: number[]
  model_filter: { type: PrivacyFilterModelFilterType; models: string[] }
  target_scope: 'all_users' | 'partial_users' | 'admin_only'
  target_user_ids: number[]
  channel_scope: 'all' | 'group' | 'channel' | 'account'
  channel_ids: number[]
  account_ids: number[]
}>({
  enabled: false,
  filter_request: true,
  filter_response: true,
  builtin_rules: {},
  custom_rules: [],
  all_groups: true,
  group_ids: [],
  model_filter: { type: 'all', models: [] },
  target_scope: 'all_users',
  target_user_ids: [],
  channel_scope: 'all',
  channel_ids: [],
  account_ids: [],
})

const targetUserSearch = ref('')
const targetUserOptions = ref<{ id: number; label: string }[]>([])
const channelOptions = ref<{ id: number; label: string }[]>([])
const accountOptions = ref<{ id: number; label: string }[]>([])
const targetUserLoading = ref(false)

const modelsText = computed({
  get: () => form.model_filter.models.join('\n'),
  set: (val: string) => {
    form.model_filter.models = val
      .split(/[\n,]+/)
      .map((m) => m.trim())
      .filter((m) => m !== '')
  },
})

const modelFilterOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.privacyFilter.scope.modelFilterAll') },
  { value: 'include', label: t('admin.privacyFilter.scope.modelFilterInclude') },
  { value: 'exclude', label: t('admin.privacyFilter.scope.modelFilterExclude') },
])

const targetScopeOptions = computed(() => [
  { value: 'all_users' as const, label: t('admin.privacyFilter.target.allUsers') },
  { value: 'partial_users' as const, label: t('admin.privacyFilter.target.partialUsers') },
  { value: 'admin_only' as const, label: t('admin.privacyFilter.target.adminOnly') },
])

const channelScopeOptions = computed(() => [
  { value: 'all' as const, label: t('admin.privacyFilter.channel.all') },
  { value: 'group' as const, label: t('admin.privacyFilter.channel.group') },
  { value: 'channel' as const, label: t('admin.privacyFilter.channel.channel') },
  { value: 'account' as const, label: t('admin.privacyFilter.channel.account') },
])

function toggleArrayId(arr: number[], id: number) {
  const idx = arr.indexOf(id)
  if (idx >= 0) arr.splice(idx, 1)
  else arr.push(id)
}

function builtinLabel(id: string): string {
  const key = `admin.privacyFilter.builtins.${id}`
  const label = t(key)
  return label === key ? id : label
}

function toggleBuiltin(id: string, checked: boolean) {
  form.builtin_rules[id] = checked
}

function toggleGroup(id: number, checked: boolean) {
  if (checked) {
    if (!form.group_ids.includes(id)) form.group_ids.push(id)
  } else {
    form.group_ids = form.group_ids.filter((g) => g !== id)
  }
}

function addCustomRule() {
  form.custom_rules.push({ name: '', pattern: '', replacement: '[REDACTED]', enabled: true })
}

function removeCustomRule(index: number) {
  form.custom_rules.splice(index, 1)
}

function applyConfig(cfg: PrivacyFilterConfig) {
  form.enabled = cfg.enabled
  form.filter_request = cfg.filter_request
  form.filter_response = cfg.filter_response
  form.builtin_rules = { ...cfg.builtin_rules }
  form.custom_rules = (cfg.custom_rules || []).map((r) => ({ ...r }))
  form.all_groups = cfg.all_groups
  form.group_ids = [...(cfg.group_ids || [])]
  form.model_filter = {
    type: cfg.model_filter?.type || 'all',
    models: [...(cfg.model_filter?.models || [])],
  }
  form.target_scope = (cfg.target_scope as any) || 'all_users'
  form.target_user_ids = [...(cfg.target_user_ids || [])]
  form.channel_scope = (cfg.channel_scope as any) || 'all'
  form.channel_ids = [...(cfg.channel_ids || [])]
  form.account_ids = [...(cfg.account_ids || [])]
  builtinIds.value = cfg.builtin_rule_ids || Object.keys(cfg.builtin_rules || {})
  // 预载已选中的目标用户显示文案
  if (form.target_user_ids.length) {
    loadTargetUserOptions(form.target_user_ids)
  }
}

async function loadTargetUserOptions(ids: number[]) {
  if (!ids.length) return
  targetUserLoading.value = true
  try {
    const res = await adminAPI.users.list(1, ids.length, { id: ids.join(',') } as any)
    targetUserOptions.value = (res.items || []).map((u: any) => ({ id: u.id, label: u.email || u.username || `#${u.id}` }))
  } catch {
    targetUserOptions.value = ids.map((id) => ({ id, label: `#${id}` }))
  } finally {
    targetUserLoading.value = false
  }
}

async function searchTargetUsers(query: string) {
  targetUserSearch.value = query
  if (!query || query.length < 1) return
  targetUserLoading.value = true
  try {
    const res = await adminAPI.users.list(1, 20, { search: query } as any)
    targetUserOptions.value = (res.items || []).map((u: any) => ({ id: u.id, label: u.email || u.username || `#${u.id}` }))
  } catch {
    /* ignore */
  } finally {
    targetUserLoading.value = false
  }
}

async function loadChannelOptions() {
  try {
    const res = await adminAPI.channels.list(1, 100)
    channelOptions.value = (res.items || []).map((c) => ({ id: c.id, label: c.name || `#${c.id}` }))
  } catch {
    channelOptions.value = []
  }
}

async function loadAccountOptions() {
  try {
    const res = await adminAPI.accounts.list(1, 50)
    accountOptions.value = (res.items || []).map((a: any) => ({ id: a.id, label: a.name || `#${a.id}` }))
  } catch {
    accountOptions.value = []
  }
}

async function load() {
  loading.value = true
  try {
    const [cfg, groupList] = await Promise.all([
      adminAPI.privacyFilter.getConfig(),
      adminAPI.groups.getAll(),
    ])
    applyConfig(cfg)
    groups.value = groupList
    // 预载渠道与账号选项（用于渠道维度选择）
    loadChannelOptions()
    loadAccountOptions()
  } catch (e) {
    statusError.value = true
    statusMessage.value = extractApiErrorMessage(e)
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  statusMessage.value = ''
  try {
    const cfg = await adminAPI.privacyFilter.updateConfig({
      enabled: form.enabled,
      filter_request: form.filter_request,
      filter_response: form.filter_response,
      builtin_rules: { ...form.builtin_rules },
      custom_rules: form.custom_rules.map((r) => ({ ...r })),
      all_groups: form.all_groups,
      group_ids: [...form.group_ids],
      model_filter: { type: form.model_filter.type, models: [...form.model_filter.models] },
      target_scope: form.target_scope,
      target_user_ids: [...form.target_user_ids],
      channel_scope: form.channel_scope,
      channel_ids: [...form.channel_ids],
      account_ids: [...form.account_ids],
    })
    applyConfig(cfg)
    statusError.value = false
    statusMessage.value = t('common.saved')
  } catch (e) {
    statusError.value = true
    statusMessage.value = extractApiErrorMessage(e)
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
