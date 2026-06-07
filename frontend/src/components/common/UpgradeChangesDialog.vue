<template>
  <BaseDialog :show="show" :title="t('version.upgradeChangesTitle')" width="wide" @close="$emit('close')">
    <div class="space-y-4">
      <div class="flex flex-wrap items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
        <span class="font-medium text-gray-900 dark:text-white">{{ displayVersion(version) }}</span>
        <a
          v-if="htmlUrl"
          :href="htmlUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
        >
          {{ t('version.viewRelease') }}
        </a>
      </div>

      <div class="max-h-[56vh] space-y-3 overflow-y-auto pr-1">
        <section
          v-for="section in displaySections"
          :key="section.key"
          class="rounded-xl border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40"
        >
          <div class="flex items-center gap-2">
            <span class="h-2 w-2 rounded-full" :class="section.dotClass"></span>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ section.title }}</h3>
          </div>
          <ul v-if="section.items.length" class="mt-3 space-y-2">
            <li
              v-for="(item, index) in section.items"
              :key="`${section.key}-${index}`"
              class="text-sm leading-6 text-gray-700 dark:text-dark-200"
            >
              {{ item }}
            </li>
          </ul>
          <p v-else class="mt-3 text-sm text-gray-500 dark:text-dark-400">
            {{ t('version.noSectionChanges') }}
          </p>
        </section>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button
          type="button"
          class="rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-dark-800"
          @click="$emit('close')"
        >
          {{ t('common.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'

type SectionKey = 'main' | 'features' | 'migration' | 'fixes'

interface ParsedSection {
  key: SectionKey
  title: string
  aliases: string[]
  dotClass: string
  items: string[]
}

const props = defineProps<{
  show: boolean
  version?: string
  body?: string
  htmlUrl?: string
}>()

defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const sectionTemplates = computed<ParsedSection[]>(() => [
  {
    key: 'main',
    title: t('version.sections.main'),
    aliases: ['主要功能', 'main', 'main features', 'major', 'highlights'],
    dotClass: 'bg-primary-500',
    items: []
  },
  {
    key: 'features',
    title: t('version.sections.features'),
    aliases: ['新增功能', '新增', 'features', 'new features', 'added'],
    dotClass: 'bg-blue-500',
    items: []
  },
  {
    key: 'migration',
    title: t('version.sections.migration'),
    aliases: ['迁移板块', '迁移', 'migration', 'migrations'],
    dotClass: 'bg-violet-500',
    items: []
  },
  {
    key: 'fixes',
    title: t('version.sections.fixes'),
    aliases: ['修复板块', '修复', 'fixes', 'bug fixes', 'fixed'],
    dotClass: 'bg-green-500',
    items: []
  }
])

const displaySections = computed(() => parseReleaseBody(props.body || ''))

function displayVersion(version?: string): string {
  const normalized = String(version || '').trim().replace(/^v/i, '')
  return normalized ? `v${normalized}` : '--'
}

function parseReleaseBody(body: string): ParsedSection[] {
  const sections = sectionTemplates.value.map((section) => ({ ...section, items: [] as string[] }))
  let active: ParsedSection | null = null
  const fallback: string[] = []

  for (const rawLine of body.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line) continue

    const heading = line.match(/^#{1,4}\s+(.+?)\s*$/)
    if (heading) {
      active = matchSection(sections, heading[1])
      continue
    }

    const label = line.match(/^(?:[-*]\s*)?(主要功能|新增功能|新增|迁移板块|迁移|修复板块|修复|Main(?: Features)?|Features|New Features|Added|Migration|Migrations|Fixes|Bug Fixes|Fixed)[:：]\s*(.*)$/i)
    if (label) {
      active = matchSection(sections, label[1])
      const value = cleanItem(label[2])
      if (active && value) active.items.push(value)
      continue
    }

    const item = cleanItem(line)
    if (!item) continue
    if (active) {
      active.items.push(item)
    } else {
      fallback.push(item)
    }
  }

  if (!sections.some((section) => section.items.length) && fallback.length) {
    sections.find((section) => section.key === 'features')?.items.push(...fallback)
  }

  return sections
}

function matchSection(sections: ParsedSection[], rawTitle: string): ParsedSection | null {
  const title = rawTitle.replace(/[*_`#]/g, '').trim().toLowerCase()
  return sections.find((section) => section.aliases.some((alias) => title.includes(alias.toLowerCase()))) || null
}

function cleanItem(raw: string): string {
  return raw
    .replace(/^[-*]\s+/, '')
    .replace(/^\d+[.)]\s+/, '')
    .replace(/^无$/, '')
    .replace(/^none$/i, '')
    .trim()
}
</script>
