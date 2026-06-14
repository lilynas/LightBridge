<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-3">
          <div class="module-stat">
            <span class="module-stat-label">{{ t('modules.installed') }}</span>
            <span class="module-stat-value">{{ installed.length }}</span>
          </div>
          <div class="module-stat">
            <span class="module-stat-label">{{ t('modules.enabled') }}</span>
            <span class="module-stat-value">{{ enabledCount }}</span>
          </div>
          <div class="module-stat">
            <span class="module-stat-label">{{ t('modules.marketplace') }}</span>
            <span class="module-stat-value">{{ marketplace.length }}</span>
          </div>
        </div>

        <button class="btn btn-secondary" :disabled="loading" @click="loadAll">
          <Icon name="refresh" size="sm" :stroke-width="2" :class="{ 'animate-spin': loading }" />
          {{ t('modules.refresh') }}
        </button>
      </div>

      <div
        v-if="error"
        class="flex items-start gap-3 rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20"
      >
        <Icon name="xCircle" size="md" :stroke-width="2" class="mt-0.5 flex-shrink-0 text-red-600 dark:text-red-400" />
        <p class="min-w-0 break-words text-sm text-red-700 dark:text-red-200">{{ error }}</p>
      </div>

      <!-- 内置功能卡片（原「功能开关」，迁移自系统设置） -->
      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('modules.builtinFeatures') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('modules.builtinFeaturesDescription') }}</p>
          </div>
          <span class="rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
            {{ t('modules.builtinCount', { count: builtinFeatures.length }) }}
          </span>
        </div>
        <div class="grid grid-cols-1 gap-4 p-5 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="feature in builtinFeatures"
            :key="feature.key"
            class="flex flex-col rounded-xl border border-gray-200 p-4 transition-colors dark:border-dark-700"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="flex min-w-0 items-center gap-3">
                <span
                  class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg"
                  :class="feature.iconBg"
                >
                  <Icon :name="(feature.icon as any)" size="sm" :stroke-width="2" />
                </span>
                <div class="min-w-0">
                  <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ feature.title }}</h3>
                  <p class="mt-0.5 line-clamp-2 text-xs text-gray-500 dark:text-dark-400">{{ feature.description }}</p>
                </div>
              </div>
              <Toggle :model-value="feature.enabled" @update:model-value="toggleBuiltinFeature(feature, $event)" />
            </div>
            <div class="mt-3 flex items-center justify-end">
              <router-link
                v-if="feature.configPath"
                :to="feature.configPath"
                class="inline-flex items-center gap-1 text-xs font-medium text-primary-600 hover:underline dark:text-primary-400"
              >
                {{ t('modules.configure') }}
                <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                </svg>
              </router-link>
            </div>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('modules.installedModules') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('modules.installedDescription') }}</p>
          </div>
          <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300">
            {{ t('modules.moduleCount', { count: installed.length }) }}
          </span>
        </div>

        <div v-if="loading" class="flex items-center justify-center py-16">
          <Icon name="refresh" size="lg" :stroke-width="2" class="animate-spin text-primary-500" />
        </div>
        <div v-else-if="installed.length === 0" class="px-5 py-12 text-center">
          <Icon name="inbox" size="xl" :stroke-width="2" class="mx-auto text-gray-400" />
          <p class="mt-3 text-sm font-medium text-gray-900 dark:text-white">{{ t('modules.noInstalled') }}</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('modules.noInstalledHint') }}</p>
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="mod in installed" :key="mod.id" class="grid gap-4 px-5 py-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ installedName(mod) }}</h3>
                <span class="module-pill">{{ moduleTypeLabel(mod.type) }}</span>
                <span class="module-pill" :class="statusClass(mod.status)">{{ statusLabel(mod.status) }}</span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                <span>{{ t('modules.versionValue', { version: mod.version }) }}</span>
                <span>{{ mod.id }}</span>
                <span v-if="mod.enabledAt">{{ t('modules.enabledAt', { time: formatDate(mod.enabledAt) }) }}</span>
              </div>
              <p v-if="mod.lastError" class="mt-2 break-words text-sm text-red-600 dark:text-red-300">{{ mod.lastError }}</p>
            </div>

            <div class="flex flex-wrap gap-2 lg:justify-end">
              <button class="btn btn-secondary px-3 py-2" :disabled="busyKey === mod.id" @click="approve(mod.id)">
                {{ t('modules.approvePermissions') }}
              </button>
              <button v-if="mod.status !== 'enabled'" class="btn btn-primary px-3 py-2" :disabled="busyKey === mod.id" @click="enable(mod.id)">
                {{ t('modules.enable') }}
              </button>
              <button v-else class="btn btn-secondary px-3 py-2" :disabled="busyKey === mod.id" @click="disable(mod.id)">
                {{ t('modules.disable') }}
              </button>
              <button class="btn btn-secondary px-3 py-2" :disabled="busyKey === mod.id" @click="uninstall(mod.id)">
                {{ t('modules.uninstall') }}
              </button>
              <button class="btn px-3 py-2 text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-900/20" :disabled="busyKey === mod.id" @click="purge(mod.id)">
                {{ t('modules.purge') }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('modules.marketplace') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('modules.marketplaceDescription') }}</p>
          </div>
          <span class="rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
            {{ t('modules.packageCount', { count: marketplace.length }) }}
          </span>
        </div>

        <div v-if="marketplace.length === 0" class="px-5 py-12 text-center">
          <Icon name="inbox" size="xl" :stroke-width="2" class="mx-auto text-gray-400" />
          <p class="mt-3 text-sm font-medium text-gray-900 dark:text-white">{{ t('modules.noMarketplace') }}</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('modules.noMarketplaceHint') }}</p>
        </div>
        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <div v-for="mod in marketplace" :key="`${mod.id}-${mod.version}`" class="grid gap-4 px-5 py-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ marketplaceName(mod) }}</h3>
                <span class="module-pill">{{ moduleTypeLabel(mod.type) }}</span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                <span>{{ t('modules.versionValue', { version: mod.version }) }}</span>
                <span>{{ mod.id }}</span>
                <span v-if="mod.sha256">{{ t('modules.signedPackage') }}</span>
              </div>
              <p v-if="marketplaceDescription(mod)" class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ marketplaceDescription(mod) }}</p>
            </div>

            <button class="btn btn-primary px-4 py-2" :disabled="busyKey === `${mod.id}:${mod.version}`" @click="install(mod.id, mod.version)">
              <Icon name="download" size="sm" :stroke-width="2" />
              {{ t('modules.install') }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import modulesAPI, { type InstalledModule, type LocalizedText, type MarketplaceModule } from '@/api/admin/modules'
import { settingsAPI } from '@/api/admin/settings'
import { useAppStore } from '@/stores'

const { t, te, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const error = ref('')
const busyKey = ref('')
const installed = ref<InstalledModule[]>([])
const marketplace = ref<MarketplaceModule[]>([])

const enabledCount = computed(() => installed.value.filter((mod) => mod.status === 'enabled').length)

// 内置功能（原系统设置「功能开关」）：通过 public-settings 标志位驱动。
const builtinFeatureBusy = ref('')
interface BuiltinFeature {
  key: string
  title: string
  description: string
  icon: string
  iconBg: string
  configPath: string
  enabled: boolean
  settingKey: string
}
const builtinFeatures = computed<BuiltinFeature[]>(() => {
  const ps = appStore.cachedPublicSettings
  return [
    {
      key: 'channel-monitor',
      title: t('modules.builtin.channelMonitor'),
      description: t('modules.builtin.channelMonitorDesc'),
      icon: 'chart',
      iconBg: 'bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300',
      configPath: '/admin/channels/monitor',
      enabled: !!ps?.channel_monitor_enabled,
      settingKey: 'channel_monitor_enabled'
    },
    {
      key: 'available-channels',
      title: t('modules.builtin.availableChannels'),
      description: t('modules.builtin.availableChannelsDesc'),
      icon: 'dollar',
      iconBg: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300',
      configPath: '/admin/channels/pricing',
      enabled: !!ps?.available_channels_enabled,
      settingKey: 'available_channels_enabled'
    },
    {
      key: 'risk-control',
      title: t('modules.builtin.riskControl'),
      description: t('modules.builtin.riskControlDesc'),
      icon: 'shield',
      iconBg: 'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300',
      configPath: '/admin/risk-control',
      enabled: !!ps?.risk_control_enabled,
      settingKey: 'risk_control_enabled'
    },
    {
      key: 'privacy-filter',
      title: t('modules.builtin.privacyFilter'),
      description: t('modules.builtin.privacyFilterDesc'),
      icon: 'shield',
      iconBg: 'bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300',
      configPath: '/admin/privacy-filter',
      enabled: !!ps?.privacy_filter_enabled,
      settingKey: 'privacy_filter_enabled'
    },
    {
      key: 'affiliate',
      title: t('modules.builtin.affiliate'),
      description: t('modules.builtin.affiliateDesc'),
      icon: 'gift',
      iconBg: 'bg-rose-50 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300',
      configPath: '/admin/affiliates/invites',
      enabled: !!ps?.affiliate_enabled,
      settingKey: 'affiliate_enabled'
    }
  ]
})

async function toggleBuiltinFeature(feature: BuiltinFeature, value: boolean) {
  if (builtinFeatureBusy.value) return
  builtinFeatureBusy.value = feature.key
  error.value = ''
  try {
    await settingsAPI.updateSettings({ [feature.settingKey]: value } as Record<string, unknown>)
    // 刷新公开设置，让侧边栏 / 路由守卫 / 卡片状态同步。
    await appStore.fetchPublicSettings(true)
  } catch (err) {
    error.value = messageOf(err)
  } finally {
    builtinFeatureBusy.value = ''
  }
}

function statusClass(status: string) {
  if (status === 'enabled') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (status === 'disabled') return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

function statusLabel(status: string) {
  const key = `modules.status.${status}`
  return te(key) ? t(key) : status
}

function moduleTypeLabel(type: string) {
  const key = `modules.type.${type}`
  return te(key) ? t(key) : type
}

function localizedText(primary: string | undefined, translations: LocalizedText | undefined, fallback: string) {
  const current = String(locale.value || '').trim()
  const candidates = [
    current,
    current.replace('_', '-'),
    current.split(/[-_]/)[0],
    current.toLowerCase().startsWith('zh') ? 'zh-CN' : '',
    current.toLowerCase().startsWith('zh') ? 'zh' : '',
    'en'
  ].filter(Boolean)
  for (const key of candidates) {
    const value = translations?.[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return primary?.trim() || fallback
}

function installedName(mod: InstalledModule) {
  const manifest = (mod.manifest || {}) as { name_i18n?: LocalizedText }
  return localizedText(mod.name, manifest.name_i18n, mod.id)
}

function marketplaceName(mod: MarketplaceModule) {
  return localizedText(mod.name, mod.name_i18n, mod.id)
}

function marketplaceDescription(mod: MarketplaceModule) {
  return localizedText(mod.summary || mod.description, mod.description_i18n, '')
}

function formatDate(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

function messageOf(err: unknown) {
  const e = err as { response?: { data?: { message?: string; error?: string } }; message?: string }
  return e.response?.data?.message || e.response?.data?.error || e.message || t('modules.operationFailed')
}

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const [installedItems, marketplaceItems] = await Promise.all([
      modulesAPI.listInstalledModules(),
      modulesAPI.listMarketplaceModules()
    ])
    installed.value = installedItems
    marketplace.value = marketplaceItems
  } catch (err) {
    error.value = messageOf(err)
  } finally {
    loading.value = false
  }
}

async function run(action: () => Promise<unknown>, key = '') {
  busyKey.value = key
  error.value = ''
  try {
    await action()
    await loadAll()
  } catch (err) {
    error.value = messageOf(err)
  } finally {
    busyKey.value = ''
  }
}

function install(id: string, version: string) {
  return run(() => modulesAPI.installMarketplaceModule(id, version), `${id}:${version}`)
}

function approve(id: string) {
  return run(() => modulesAPI.approveModulePermissions(id), id)
}

function enable(id: string) {
  return run(() => modulesAPI.enableModule(id), id)
}

function disable(id: string) {
  return run(() => modulesAPI.disableModule(id), id)
}

function uninstall(id: string) {
  return run(() => modulesAPI.uninstallModule(id), id)
}

function purge(id: string) {
  return run(() => modulesAPI.purgeModule(id), id)
}

onMounted(loadAll)
</script>

<style scoped>
.module-stat {
  @apply flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-800;
}

.module-stat-label {
  @apply text-xs font-medium text-gray-500 dark:text-dark-400;
}

.module-stat-value {
  @apply text-sm font-semibold text-gray-900 dark:text-white;
}

.module-pill {
  @apply rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-200;
}
</style>
