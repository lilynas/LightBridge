<template>
  <div class="docs-shell min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="sticky top-0 z-20 border-b border-gray-200 bg-white/95 backdrop-blur dark:border-dark-800 dark:bg-dark-950/95">
      <div class="flex h-16 items-center justify-between px-4 md:px-6">
        <router-link
          to="/dashboard"
          class="inline-flex items-center gap-2 rounded-lg px-2.5 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white"
        >
          <Icon name="arrowLeft" size="sm" />
          <span>{{ t('common.back') }}</span>
        </router-link>

        <div class="flex items-center gap-2 text-sm font-semibold text-gray-800 dark:text-dark-100">
          <Icon name="book" size="sm" class="text-primary-600 dark:text-primary-400" />
          <span>{{ t('nav.docs') }}</span>
        </div>

        <div class="w-16" aria-hidden="true"></div>
      </div>
    </header>

    <div class="docs-grid mx-auto grid max-w-[1680px] grid-cols-1 lg:grid-cols-[280px_minmax(0,1fr)_260px]">
      <aside class="border-b border-gray-200 bg-white px-4 py-4 dark:border-dark-800 dark:bg-dark-900/70 lg:sticky lg:top-16 lg:h-[calc(100vh-4rem)] lg:border-b-0 lg:border-r lg:py-6">
        <nav class="flex gap-2 overflow-x-auto lg:block lg:space-y-1 lg:overflow-visible" :aria-label="t('docs.documentList')">
          <button
            v-for="doc in docs"
            :key="doc.id"
            type="button"
            class="docs-list-item shrink-0 rounded-lg px-3 py-2 text-left text-sm transition-colors lg:w-full"
            :class="doc.id === selectedDoc.id
              ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/25 dark:text-primary-300'
              : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white'"
            @click="selectDoc(doc.id)"
          >
            <span class="block font-medium">{{ doc.title }}</span>
            <span v-if="doc.description" class="mt-1 hidden text-xs text-gray-500 dark:text-dark-400 lg:block">{{ doc.description }}</span>
          </button>
        </nav>
      </aside>

      <main ref="contentRef" class="docs-content min-h-[calc(100vh-8rem)] overflow-auto px-5 py-8 md:px-8 lg:h-[calc(100vh-4rem)] lg:px-12 lg:py-10" @scroll="updateActiveHeading">
        <article class="markdown-doc mx-auto max-w-4xl" v-html="renderedHtml"></article>
      </main>

      <aside class="hidden border-l border-gray-200 bg-white px-5 py-6 dark:border-dark-800 dark:bg-dark-900/70 lg:sticky lg:top-16 lg:block lg:h-[calc(100vh-4rem)]">
        <div class="text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-dark-500">
          {{ t('docs.toc') }}
        </div>
        <nav v-if="tocItems.length" class="mt-4 space-y-1" :aria-label="t('docs.toc')">
          <button
            v-for="item in tocItems"
            :key="item.id"
            type="button"
            class="block w-full rounded-md py-1.5 pr-2 text-left text-sm transition-colors"
            :class="[
              item.level >= 3 ? 'pl-5' : 'pl-2',
              activeHeadingId === item.id
                ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/25 dark:text-primary-300'
                : 'text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white'
            ]"
            @click="scrollToHeading(item.id)"
          >
            {{ item.text }}
          </button>
        </nav>
        <p v-else class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('docs.noToc') }}</p>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import { findDocById, lightBridgeDocs } from '@/docs'

interface TocItem {
  id: string
  text: string
  level: number
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const docs = lightBridgeDocs
const contentRef = ref<HTMLElement | null>(null)
const tocItems = ref<TocItem[]>([])
const activeHeadingId = ref('')

marked.setOptions({
  breaks: true,
  gfm: true,
})

const selectedDoc = computed(() => {
  const queryDoc = Array.isArray(route.query.doc) ? route.query.doc[0] : route.query.doc
  return findDocById(queryDoc)
})

const renderedHtml = computed(() => {
  const html = marked.parse(selectedDoc.value.content) as string
  const sanitized = DOMPurify.sanitize(html)
  return injectHeadingIds(sanitized)
})

function generateHeadingId(text: string, index: number): string {
  const base = text
    .toLowerCase()
    .replace(/[^\w\u4e00-\u9fff]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return base ? `${base}-${index}` : `heading-${index}`
}

function injectHeadingIds(html: string): string {
  const toc: TocItem[] = []
  let headingIndex = 0
  const withIds = html.replace(/<(h[1-3])[^>]*>(.*?)<\/h[1-3]>/gi, (_, tag: string, content: string) => {
    const level = Number(tag[1])
    const text = content.replace(/<[^>]+>/g, '').trim()
    const id = generateHeadingId(text, headingIndex++)
    if (level >= 2) {
      toc.push({ id, text, level })
    }
    return `<${tag} id="${id}">${content}</${tag}>`
  })
  tocItems.value = toc
  activeHeadingId.value = toc[0]?.id ?? ''
  return withIds
}

async function selectDoc(id: string) {
  await router.replace({ query: { ...route.query, doc: id } })
}

function scrollToHeading(id: string) {
  const container = contentRef.value
  if (!container) return
  const heading = container.querySelector(`#${CSS.escape(id)}`)
  if (!heading) return
  heading.scrollIntoView({ behavior: 'smooth', block: 'start' })
  activeHeadingId.value = id
}

let scrollRafId = 0
function updateActiveHeading() {
  if (scrollRafId) return
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = 0
    const container = contentRef.value
    if (!container) return

    const containerTop = container.getBoundingClientRect().top
    let current = tocItems.value[0]?.id ?? ''
    for (const item of tocItems.value) {
      const heading = container.querySelector(`#${CSS.escape(item.id)}`) as HTMLElement | null
      if (heading && heading.getBoundingClientRect().top - containerTop <= 96) {
        current = item.id
      }
    }
    activeHeadingId.value = current
  })
}

watch(
  () => selectedDoc.value.id,
  async () => {
    await nextTick()
    if (contentRef.value) {
      contentRef.value.scrollTop = 0
    }
  }
)
</script>

<style scoped>
.docs-list-item {
  min-width: 168px;
}

@media (min-width: 1024px) {
  .docs-list-item {
    min-width: 0;
  }
}
</style>

<style>
.markdown-doc {
  line-height: 1.75;
  color: inherit;
}

.markdown-doc h1 { @apply mb-6 border-b border-gray-200 pb-4 text-3xl font-bold text-gray-950 dark:border-dark-700 dark:text-white; }
.markdown-doc h2 { @apply mt-10 mb-4 text-2xl font-bold text-gray-950 scroll-mt-24 dark:text-white; }
.markdown-doc h3 { @apply mt-8 mb-3 text-xl font-semibold text-gray-900 scroll-mt-24 dark:text-dark-100; }
.markdown-doc p { @apply mb-4 text-gray-700 dark:text-dark-200; }
.markdown-doc ul { @apply mb-4 list-disc pl-6 text-gray-700 dark:text-dark-200; }
.markdown-doc ol { @apply mb-4 list-decimal pl-6 text-gray-700 dark:text-dark-200; }
.markdown-doc li { @apply mb-1; }
.markdown-doc a { @apply text-primary-600 underline underline-offset-2 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300; }
.markdown-doc blockquote { @apply my-5 border-l-4 border-primary-200 bg-primary-50/50 px-4 py-3 text-gray-700 dark:border-primary-800 dark:bg-primary-900/15 dark:text-dark-200; }
.markdown-doc img { @apply my-6 h-auto max-w-full rounded-lg border border-gray-200 dark:border-dark-700; }
.markdown-doc table { @apply my-6 w-full border-collapse overflow-hidden rounded-lg text-sm; }
.markdown-doc th { @apply border border-gray-200 bg-gray-100 px-3 py-2 text-left font-semibold text-gray-900 dark:border-dark-700 dark:bg-dark-800 dark:text-white; }
.markdown-doc td { @apply border border-gray-200 px-3 py-2 text-gray-700 dark:border-dark-700 dark:text-dark-200; }
.markdown-doc code { @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-sm text-gray-900 dark:bg-dark-800 dark:text-dark-100; }
.markdown-doc pre { @apply my-5 overflow-x-auto rounded-lg bg-gray-950 p-4 text-gray-100; }
.markdown-doc pre code { @apply bg-transparent p-0 text-inherit; }
.markdown-doc hr { @apply my-8 border-gray-200 dark:border-dark-700; }
</style>
