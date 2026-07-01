<template>
  <div class="docs-shell min-h-screen bg-white text-gray-950 dark:bg-dark-950 dark:text-white">
    <header class="sticky top-0 z-20 border-b border-red-100 bg-white/95 backdrop-blur dark:border-red-900/30 dark:bg-dark-950/95">
      <div class="flex h-16 items-center justify-between px-4 md:px-6">
        <router-link
          to="/home"
          class="inline-flex items-center gap-3 rounded-lg px-2 py-1.5 transition-colors hover:bg-red-50 dark:hover:bg-red-950/30"
        >
          <span class="sidebar-logo flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl shadow-glow">
            <img :src="siteLogo || '/logo.png'" alt="LightBridge" class="h-full w-full object-contain" />
          </span>
          <span class="text-xl font-light text-gray-950 dark:text-white">{{ siteName }}</span>
        </router-link>

        <router-link
          :to="consolePath"
          class="inline-flex h-9 items-center justify-center rounded-lg bg-red-600 px-4 text-sm font-semibold text-white shadow-sm shadow-red-600/20 transition-colors hover:bg-red-700"
        >
          {{ t('docs.goToConsole') }}
        </router-link>
      </div>
    </header>

    <div
      class="docs-grid grid w-full grid-cols-1 transition-all duration-300 lg:grid-cols-[var(--docs-sidebar-width)_minmax(0,1fr)] 2xl:grid-cols-[var(--docs-sidebar-width)_minmax(0,1fr)_190px]"
      :style="{ '--docs-sidebar-width': docsCollapsed ? '64px' : '260px' }"
    >
      <aside class="border-b border-red-100 bg-white transition-all duration-300 dark:border-red-900/30 dark:bg-dark-950 lg:sticky lg:top-16 lg:h-[calc(100vh-4rem)] lg:border-b-0 lg:border-r">
        <div class="flex items-center justify-between px-4 py-4">
          <span v-if="!docsCollapsed" class="text-xs font-semibold uppercase tracking-wide text-gray-900 dark:text-white">
            {{ t('docs.documentList') }}
          </span>
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-red-50 hover:text-red-700 dark:text-dark-400 dark:hover:bg-red-950/30 dark:hover:text-red-300"
            :aria-label="docsCollapsed ? t('common.expand') : t('common.collapse')"
            @click="docsCollapsed = !docsCollapsed"
          >
            <Icon :name="docsCollapsed ? 'chevronRight' : 'chevronLeft'" size="sm" />
          </button>
        </div>

        <nav v-if="!docsCollapsed" class="space-y-5 px-4 pb-5" :aria-label="t('docs.documentList')">
          <section v-for="group in groupedDocs" :key="group.name">
            <div class="mb-2 text-xs font-semibold text-gray-500 dark:text-dark-400">{{ group.name }}</div>
            <div class="space-y-1">
              <button
                v-for="doc in group.docs"
                :key="doc.id"
                type="button"
                class="docs-list-item block w-full rounded-md border-l-2 px-3 py-2 text-left text-sm font-medium transition-all"
                :class="doc.id === selectedDoc.id
                  ? 'border-red-600 bg-red-50 text-gray-950 dark:border-red-400 dark:bg-red-950/30 dark:text-white'
                  : 'border-transparent text-gray-700 hover:border-red-200 hover:bg-red-50/70 hover:text-gray-950 dark:text-dark-300 dark:hover:border-red-900/60 dark:hover:bg-red-950/20 dark:hover:text-white'"
                @click="selectDoc(doc.id)"
              >
                {{ doc.title }}
              </button>
            </div>
          </section>
        </nav>

        <div v-else class="flex justify-center px-3 pb-5">
          <Icon name="book" size="md" class="text-red-400 dark:text-red-300/70" />
        </div>
      </aside>

      <main ref="contentRef" class="docs-content min-h-[calc(100vh-8rem)] overflow-auto bg-white px-5 py-8 dark:bg-dark-950 md:px-8 lg:h-[calc(100vh-4rem)] lg:px-12 lg:py-10" @scroll="updateActiveHeading">
        <article class="markdown-doc w-full" v-html="renderedHtml"></article>
      </main>

      <aside class="hidden bg-white dark:bg-dark-950 2xl:sticky 2xl:top-16 2xl:flex 2xl:h-[calc(100vh-4rem)] 2xl:items-end 2xl:pb-8 2xl:pr-7">
        <nav v-if="tocItems.length" class="floating-toc" :aria-label="t('docs.toc')">
          <div class="mb-2 text-xs font-semibold text-gray-400 dark:text-dark-500">{{ t('docs.toc') }}</div>
          <button
            v-for="item in tocItems"
            :key="item.id"
            type="button"
            class="floating-toc-item"
            :class="[
              item.level >= 3 ? 'pl-4' : 'pl-0',
              activeHeadingId === item.id ? 'active' : ''
            ]"
            @click="scrollToHeading(item.id)"
          >
            {{ item.text }}
          </button>
        </nav>
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
import { useAppStore, useAuthStore } from '@/stores'

interface TocItem {
  id: string
  text: string
  level: number
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const docs = lightBridgeDocs
const contentRef = ref<HTMLElement | null>(null)
const tocItems = ref<TocItem[]>([])
const activeHeadingId = ref('')
const docsCollapsed = ref(false)

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'LightBridge')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const consolePath = computed(() => {
  if (!authStore.isAuthenticated) return '/login'
  return authStore.isAdmin ? '/admin/dashboard' : '/dashboard'
})

const groupedDocs = computed(() => {
  const groups = new Map<string, typeof docs>()
  for (const doc of docs) {
    const group = doc.group || t('common.all')
    groups.set(group, [...(groups.get(group) ?? []), doc])
  }
  return Array.from(groups, ([name, groupDocs]) => ({ name, docs: groupDocs }))
})

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
.docs-grid {
  --docs-sidebar-width: 260px;
}

.docs-list-item {
  transform: translateX(0);
}

.docs-list-item:hover {
  transform: translateX(2px);
}

.floating-toc {
  width: 100%;
  max-height: min(48vh, 420px);
  overflow: auto;
  padding: 4px 0;
  animation: toc-enter 0.28s ease-out both;
}

.floating-toc-item {
  display: block;
  width: 100%;
  padding-top: 6px;
  padding-bottom: 6px;
  text-align: left;
  font-size: 13px;
  line-height: 1.35;
  color: rgb(107 114 128);
  transition: color 0.18s ease, transform 0.18s ease, opacity 0.18s ease;
}

.floating-toc-item:hover {
  color: rgb(185 28 28);
  transform: translateX(4px);
}

.floating-toc-item.active {
  color: rgb(220 38 38);
  font-weight: 600;
  transform: translateX(6px);
}

.dark .floating-toc-item {
  color: rgb(156 163 175);
}

.dark .floating-toc-item:hover,
.dark .floating-toc-item.active {
  color: rgb(252 165 165);
}

@keyframes toc-enter {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>

<style>
.markdown-doc {
  line-height: 1.75;
  color: inherit;
}

.markdown-doc h1 { @apply mb-6 border-b border-red-100 pb-4 text-3xl font-bold text-gray-950 dark:border-red-900/30 dark:text-white; }
.markdown-doc h2 { @apply mt-10 mb-4 text-2xl font-bold text-gray-950 scroll-mt-24 dark:text-white; }
.markdown-doc h3 { @apply mt-8 mb-3 text-xl font-semibold text-gray-900 scroll-mt-24 dark:text-dark-100; }
.markdown-doc p { @apply mb-4 text-gray-700 dark:text-dark-200; }
.markdown-doc ul { @apply mb-4 list-disc pl-6 text-gray-700 dark:text-dark-200; }
.markdown-doc ol { @apply mb-4 list-decimal pl-6 text-gray-700 dark:text-dark-200; }
.markdown-doc li { @apply mb-1; }
.markdown-doc a { @apply text-red-600 underline underline-offset-2 hover:text-red-700 dark:text-red-300 dark:hover:text-red-200; }
.markdown-doc blockquote { @apply my-5 border-l-4 border-red-200 bg-red-50/70 px-4 py-3 text-gray-700 dark:border-red-900/70 dark:bg-red-950/20 dark:text-dark-200; }
.markdown-doc img { @apply my-6 h-auto max-w-full rounded-lg border border-red-100 dark:border-red-900/40; }
.markdown-doc table { @apply my-6 w-full border-collapse overflow-hidden rounded-lg text-sm; }
.markdown-doc th { @apply border border-red-100 bg-red-50 px-3 py-2 text-left font-semibold text-gray-900 dark:border-red-900/40 dark:bg-red-950/30 dark:text-white; }
.markdown-doc td { @apply border border-red-100 px-3 py-2 text-gray-700 dark:border-red-900/40 dark:text-dark-200; }
.markdown-doc code { @apply rounded bg-red-50 px-1.5 py-0.5 font-mono text-sm text-gray-900 dark:bg-red-950/30 dark:text-dark-100; }
.markdown-doc pre { @apply my-5 overflow-x-auto rounded-lg bg-gray-950 p-4 text-gray-100; }
.markdown-doc pre code { @apply bg-transparent p-0 text-inherit; }
.markdown-doc hr { @apply my-8 border-red-100 dark:border-red-900/30; }
</style>
