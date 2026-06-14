<template>
  <div class="card">
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('admin.ops.availability.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          {{ t('admin.ops.availability.description') }}
        </p>
      </div>
      <button class="btn btn-secondary px-3 py-1.5 text-sm" :disabled="loading" @click="load">
        <Icon name="refresh" size="sm" :stroke-width="2" :class="{ 'animate-spin': loading }" />
        {{ t('common.refresh') }}
      </button>
    </div>

    <div class="p-5">
      <div v-if="loading && !points.length" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="lg" :stroke-width="2" class="animate-spin text-primary-500" />
      </div>

      <div v-else-if="error" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800/40 dark:bg-amber-900/20 dark:text-amber-300">
        {{ error }}
      </div>

      <template v-else>
        <!-- 网格：每列一周（7 天），共约 30 天 -->
        <div class="overflow-x-auto">
          <div class="flex items-end gap-3">
            <div class="flex flex-col justify-between pb-5 text-[10px] text-gray-400 dark:text-dark-400">
              <span>Mon</span>
              <span>Wed</span>
              <span>Fri</span>
            </div>
            <div class="flex gap-1">
              <div v-for="(week, wi) in weeks" :key="wi" class="flex flex-col gap-1">
                <div
                  v-for="(day, di) in week"
                  :key="`${wi}-${di}`"
                  class="group relative h-3.5 w-3.5 rounded-sm transition-transform hover:scale-110"
                  :class="day ? cellClass(day.availability, day.total_checks) : 'bg-transparent'"
                >
                  <div
                    v-if="day"
                    class="pointer-events-none absolute bottom-full left-1/2 z-20 mb-1 hidden -translate-x-1/2 whitespace-nowrap rounded bg-gray-900 px-2 py-1 text-[11px] text-white group-hover:block dark:bg-gray-700"
                  >
                    {{ formatDay(day.date) }}：{{ formatPct(day.availability) }}
                    <span class="text-gray-300">（{{ day.total_checks }} 次检测）</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 图例 -->
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
            <span>{{ t('admin.ops.availability.less') }}</span>
            <span class="h-3.5 w-3.5 rounded-sm bg-gray-100 dark:bg-dark-700" />
            <span class="h-3.5 w-3.5 rounded-sm" :class="cellClass(0.5, 1)" />
            <span class="h-3.5 w-3.5 rounded-sm" :class="cellClass(0.8, 1)" />
            <span class="h-3.5 w-3.5 rounded-sm" :class="cellClass(0.95, 1)" />
            <span class="h-3.5 w-3.5 rounded-sm" :class="cellClass(1, 1)" />
            <span>{{ t('admin.ops.availability.more') }}</span>
          </div>
          <div class="flex items-center gap-4 text-sm">
            <div>
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.ops.availability.avg30d') }}：</span>
              <span class="font-semibold text-gray-900 dark:text-white">{{ formatPct(avgAvailability) }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-dark-400">{{ t('admin.ops.availability.totalChecks') }}：</span>
              <span class="font-semibold text-gray-900 dark:text-white">{{ totalChecks.toLocaleString() }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { DailyAvailabilityPoint } from '@/api/admin/availability'

const { t } = useI18n()

const props = withDefaults(defineProps<{ days?: number }>(), { days: 30 })

const loading = ref(false)
const error = ref('')
const points = ref<DailyAvailabilityPoint[]>([])

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await adminAPI.availability.getRecentAvailability(props.days)
    points.value = res.points || []
  } catch (e: any) {
    error.value = e?.response?.data?.message || e?.message || t('common.error')
  } finally {
    loading.value = false
  }
}

// 把线性 points 按"周列"重组（每列 7 天，对齐到周起始），便于 GitHub 风格渲染。
const weeks = computed<(DailyAvailabilityPoint | null)[][]>(() => {
  const list = points.value
  if (!list.length) return []
  const first = new Date(list[0].date)
  const firstDow = first.getDay() // 0=Sun ... 6=Sat
  const padded: (DailyAvailabilityPoint | null)[] = []
  for (let i = 0; i < firstDow; i++) padded.push(null)
  padded.push(...list.map((p) => ({ ...p, date: p.date })))
  const cols: (DailyAvailabilityPoint | null)[][] = []
  for (let i = 0; i < padded.length; i += 7) {
    cols.push(padded.slice(i, i + 7))
  }
  return cols
})

const totalChecks = computed(() => points.value.reduce((s, p) => s + (p.total_checks || 0), 0))
const totalOk = computed(() => points.value.reduce((s, p) => s + (p.ok_count || 0), 0))
const avgAvailability = computed(() => (totalChecks.value > 0 ? totalOk.value / totalChecks.value : 0))

function cellClass(avail: number, total: number): string {
  if (total === 0) return 'bg-gray-100 dark:bg-dark-700'
  if (avail >= 0.99) return 'bg-emerald-500'
  if (avail >= 0.95) return 'bg-emerald-400'
  if (avail >= 0.85) return 'bg-yellow-400'
  if (avail >= 0.6) return 'bg-orange-400'
  return 'bg-rose-500'
}

function formatDay(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function formatPct(v: number): string {
  return `${(v * 100).toFixed(1)}%`
}

onMounted(load)
</script>
