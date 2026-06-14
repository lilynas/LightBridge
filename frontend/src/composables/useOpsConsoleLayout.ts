import { computed, ref, watch } from 'vue'

/**
 * 管理控制台自定义卡片布局。
 *
 * 维护一组"启用卡片"的有序列表，持久化到 localStorage。
 * 默认启用全部已知卡片；用户可在自定义面板里勾选/排序。
 */
const STORAGE_KEY = 'lb-ops-console-layout'

export interface ConsoleCardDef {
  key: string
  /** 显示名称（i18n key 或字面量） */
  label: string
  /** 默认是否启用 */
  defaultEnabled?: boolean
}

// 已知卡片清单（顺序即默认顺序）。新增卡片时在此登记。
export const KNOWN_OPS_CARDS: ConsoleCardDef[] = [
  { key: 'availability', label: 'admin.ops.cards.availability', defaultEnabled: true },
  { key: 'concurrency', label: 'admin.ops.cards.concurrency', defaultEnabled: true },
  { key: 'switchRateTrend', label: 'admin.ops.cards.switchRateTrend', defaultEnabled: true },
  { key: 'throughputTrend', label: 'admin.ops.cards.throughputTrend', defaultEnabled: true },
  { key: 'latency', label: 'admin.ops.cards.latency', defaultEnabled: true },
  { key: 'errorDistribution', label: 'admin.ops.cards.errorDistribution', defaultEnabled: true },
  { key: 'errorTrend', label: 'admin.ops.cards.errorTrend', defaultEnabled: true },
  { key: 'openaiTokenStats', label: 'admin.ops.cards.openaiTokenStats', defaultEnabled: true },
  { key: 'alertEvents', label: 'admin.ops.cards.alertEvents', defaultEnabled: true },
  { key: 'systemLog', label: 'admin.ops.cards.systemLog', defaultEnabled: true },
  { key: 'modelDistribution', label: 'admin.ops.cards.modelDistribution', defaultEnabled: false },
  { key: 'tokenUsageTrend', label: 'admin.ops.cards.tokenUsageTrend', defaultEnabled: false },
  { key: 'todayRequests', label: 'admin.ops.cards.todayRequests', defaultEnabled: false },
  { key: 'totalTokens', label: 'admin.ops.cards.totalTokens', defaultEnabled: false },
  { key: 'avgResponseTime', label: 'admin.ops.cards.avgResponseTime', defaultEnabled: false }
]

function loadStored(): string[] | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed) && parsed.every((x) => typeof x === 'string')) return parsed as string[]
  } catch {
    /* ignore */
  }
  return null
}

function defaultOrder(): string[] {
  return KNOWN_OPS_CARDS.filter((c) => c.defaultEnabled).map((c) => c.key)
}

const persisted = loadStored()
// 全局单例状态（整个应用共享同一份布局）
const enabledCards = ref<string[]>(persisted && persisted.length ? persisted : defaultOrder())

watch(
  enabledCards,
  (val) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(val))
    } catch {
      /* ignore */
    }
  },
  { deep: true }
)

export function useOpsConsoleLayout() {
  const enabled = computed(() => enabledCards.value)
  const allCards = KNOWN_OPS_CARDS

  function isEnabled(key: string): boolean {
    return enabledCards.value.includes(key)
  }

  function toggle(key: string) {
    const idx = enabledCards.value.indexOf(key)
    if (idx >= 0) {
      enabledCards.value = enabledCards.value.filter((k) => k !== key)
    } else {
      enabledCards.value = [...enabledCards.value, key]
    }
  }

  function moveUp(key: string) {
    const idx = enabledCards.value.indexOf(key)
    if (idx > 0) {
      const next = [...enabledCards.value]
      ;[next[idx - 1], next[idx]] = [next[idx], next[idx - 1]]
      enabledCards.value = next
    }
  }

  function moveDown(key: string) {
    const idx = enabledCards.value.indexOf(key)
    if (idx >= 0 && idx < enabledCards.value.length - 1) {
      const next = [...enabledCards.value]
      ;[next[idx + 1], next[idx]] = [next[idx], next[idx + 1]]
      enabledCards.value = next
    }
  }

  function reset() {
    enabledCards.value = defaultOrder()
  }

  return { enabled, allCards, isEnabled, toggle, moveUp, moveDown, reset }
}
