import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

/**
 * 全局时间范围 / 颗粒度 store。
 *
 * 由顶部菜单栏的"时间范围"按钮驱动，供仪表盘与管理控制台共享同一份选择。
 * 选择会持久化到 localStorage，刷新后保持。
 */
const STORAGE_KEY = 'lb-time-range'
const GRANULARITY_STORAGE_KEY = 'lb-time-granularity'

export type DashboardGranularity = 'hour' | 'day'

interface StoredRange {
  start: string // YYYY-MM-DD
  end: string
}

function defaultRange(): StoredRange {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - 29)
  return { start: toDateString(start), end: toDateString(end) }
}

function toDateString(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function loadStored(): StoredRange {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return defaultRange()
    const parsed = JSON.parse(raw) as Partial<StoredRange>
    if (typeof parsed.start === 'string' && typeof parsed.end === 'string') {
      return { start: parsed.start, end: parsed.end }
    }
  } catch {
    /* ignore */
  }
  return defaultRange()
}

function loadGranularity(): DashboardGranularity {
  const raw = localStorage.getItem(GRANULARITY_STORAGE_KEY)
  return raw === 'day' ? 'day' : 'hour'
}

export const useTimeRangeStore = defineStore('timeRange', () => {
  const stored = loadStored()
  const startDate = ref<string>(stored.start)
  const endDate = ref<string>(stored.end)
  const granularity = ref<DashboardGranularity>(loadGranularity())

  const startISO = computed(() => `${startDate.value}T00:00:00`)
  const endISO = computed(() => `${endDate.value}T23:59:59`)

  function setRange(start: string, end: string) {
    startDate.value = start
    endDate.value = end
  }

  function setGranularity(g: DashboardGranularity) {
    granularity.value = g
  }

  function reset() {
    const def = defaultRange()
    startDate.value = def.start
    endDate.value = def.end
    granularity.value = 'hour'
  }

  watch(
    [startDate, endDate],
    ([s, e]) => {
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ start: s, end: e }))
      } catch {
        /* ignore */
      }
    },
    { deep: true }
  )

  watch(granularity, (g) => {
    try {
      localStorage.setItem(GRANULARITY_STORAGE_KEY, g)
    } catch {
      /* ignore */
    }
  })

  return {
    startDate,
    endDate,
    granularity,
    startISO,
    endISO,
    setRange,
    setGranularity,
    reset
  }
})
