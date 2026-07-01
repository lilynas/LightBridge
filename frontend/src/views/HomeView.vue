<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div v-else class="min-h-screen overflow-hidden bg-white text-gray-950 dark:bg-dark-950 dark:text-white">
    <header class="relative z-20 border-b border-red-100/80 bg-white/95 px-5 py-4 backdrop-blur dark:border-red-900/30 dark:bg-dark-950/95">
      <nav class="mx-auto flex max-w-7xl items-center justify-between">
        <router-link to="/home" class="flex items-center gap-3">
          <span class="flex h-10 w-10 overflow-hidden rounded-xl border border-red-100 bg-white shadow-sm dark:border-red-900/50 dark:bg-dark-900">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="hidden text-sm font-semibold tracking-wide text-gray-900 dark:text-white sm:inline">{{ siteName }}</span>
        </router-link>

        <div class="flex items-center gap-2">
          <router-link
            to="/docs"
            class="flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition-all hover:scale-105 hover:bg-red-50 hover:text-red-700 dark:text-gray-400 dark:hover:bg-red-950/40 dark:hover:text-red-300"
            :title="t('home.viewDocs')"
            :aria-label="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </router-link>
          <LocaleSwitcher />
          <button
            @click="toggleTheme"
            class="flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition-all hover:scale-105 hover:bg-red-50 hover:text-red-700 dark:text-gray-400 dark:hover:bg-red-950/40 dark:hover:text-red-300"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex h-9 items-center rounded-lg bg-red-600 px-4 text-sm font-semibold text-white shadow-sm shadow-red-600/20 transition-colors hover:bg-red-700"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main>
      <section class="relative border-b border-red-100 bg-white dark:border-red-900/30 dark:bg-dark-950">
        <div class="absolute inset-0 bg-[linear-gradient(rgba(220,38,38,0.07)_1px,transparent_1px),linear-gradient(90deg,rgba(220,38,38,0.07)_1px,transparent_1px)] bg-[size:48px_48px]"></div>
        <div class="relative mx-auto grid min-h-[calc(100vh-73px)] max-w-7xl items-center gap-12 px-5 py-16 lg:grid-cols-[1fr_520px] lg:py-20">
          <div class="max-w-3xl">
            <div class="mb-5 inline-flex items-center gap-2 rounded-full border border-red-200 bg-red-50 px-3 py-1 text-xs font-semibold text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300">
              <span class="h-1.5 w-1.5 rounded-full bg-red-600"></span>
              {{ t('home.heroSubtitle') }}
            </div>
            <h1 class="text-5xl font-bold leading-tight tracking-normal text-gray-950 dark:text-white md:text-6xl lg:text-7xl">
              {{ siteName }}
            </h1>
            <p class="mt-6 max-w-2xl text-lg leading-8 text-gray-600 dark:text-dark-300 md:text-xl">
              {{ siteSubtitle }}
            </p>
            <div class="mt-9 flex flex-col gap-3 sm:flex-row">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="inline-flex h-12 items-center justify-center rounded-lg bg-red-600 px-6 text-sm font-semibold text-white shadow-lg shadow-red-600/20 transition-colors hover:bg-red-700"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" class="ml-2" :stroke-width="2" />
              </router-link>
              <router-link
                to="/docs"
                class="inline-flex h-12 items-center justify-center rounded-lg border border-red-200 bg-white px-6 text-sm font-semibold text-red-700 transition-colors hover:bg-red-50 dark:border-red-900/60 dark:bg-dark-900 dark:text-red-300 dark:hover:bg-red-950/30"
              >
                <Icon name="book" size="sm" class="mr-2" />
                {{ t('home.viewDocs') }}
              </router-link>
            </div>
          </div>

          <div class="relative">
            <div class="absolute -left-4 top-4 h-full w-full border-2 border-red-200 dark:border-red-900/50"></div>
            <div class="relative overflow-hidden rounded-xl border border-red-100 bg-gray-950 shadow-2xl shadow-red-950/20 dark:border-red-900/50">
              <div class="flex items-center justify-between border-b border-white/10 bg-red-600 px-4 py-3">
                <div class="flex gap-2">
                  <span class="h-3 w-3 rounded-full bg-white/90"></span>
                  <span class="h-3 w-3 rounded-full bg-white/60"></span>
                  <span class="h-3 w-3 rounded-full bg-white/40"></span>
                </div>
                <span class="font-mono text-xs font-semibold text-white/90">LightBridge API</span>
              </div>
              <div class="space-y-4 p-5 font-mono text-sm leading-7 text-gray-100 md:p-7">
                <div class="code-line line-1"><span class="text-red-300">$</span> curl -X POST /v1/chat/completions</div>
                <div class="code-line line-2 text-gray-400">routing: claude -> gpt -> gemini</div>
                <div class="code-line line-3"><span class="rounded bg-red-500/20 px-2 py-0.5 text-red-200">200 OK</span> unified response ready</div>
                <div class="code-line line-4 flex items-center gap-2"><span class="text-red-300">$</span><span class="cursor"></span></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="bg-red-50/60 px-5 py-14 dark:bg-red-950/10">
        <div class="mx-auto grid max-w-7xl gap-5 md:grid-cols-3">
          <div class="rounded-lg border border-red-100 bg-white p-6 shadow-sm dark:border-red-900/40 dark:bg-dark-900">
            <Icon name="server" size="lg" class="text-red-600 dark:text-red-300" />
            <h2 class="mt-4 text-lg font-semibold text-gray-950 dark:text-white">{{ t('home.features.unifiedGateway') }}</h2>
            <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('home.features.unifiedGatewayDesc') }}</p>
          </div>
          <div class="rounded-lg border border-red-100 bg-white p-6 shadow-sm dark:border-red-900/40 dark:bg-dark-900">
            <Icon name="shield" size="lg" class="text-red-600 dark:text-red-300" />
            <h2 class="mt-4 text-lg font-semibold text-gray-950 dark:text-white">{{ t('home.features.multiAccount') }}</h2>
            <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('home.features.multiAccountDesc') }}</p>
          </div>
          <div class="rounded-lg border border-red-100 bg-white p-6 shadow-sm dark:border-red-900/40 dark:bg-dark-900">
            <Icon name="chart" size="lg" class="text-red-600 dark:text-red-300" />
            <h2 class="mt-4 text-lg font-semibold text-gray-950 dark:text-white">{{ t('home.features.balanceQuota') }}</h2>
            <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('home.features.balanceQuotaDesc') }}</p>
          </div>
        </div>
      </section>

      <section class="bg-white px-5 py-16 dark:bg-dark-950">
        <div class="mx-auto max-w-7xl">
          <div class="flex flex-col justify-between gap-5 md:flex-row md:items-end">
            <div>
              <h2 class="text-3xl font-bold text-gray-950 dark:text-white">{{ t('home.providers.title') }}</h2>
              <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ t('home.providers.description') }}</p>
            </div>
            <router-link to="/docs" class="inline-flex h-10 items-center justify-center rounded-lg border border-red-200 px-4 text-sm font-semibold text-red-700 hover:bg-red-50 dark:border-red-900/60 dark:text-red-300 dark:hover:bg-red-950/30">
              <Icon name="book" size="sm" class="mr-2" />
              {{ t('home.docs') }}
            </router-link>
          </div>

          <div class="mt-8 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <div v-for="provider in providers" :key="provider.name" class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-800 dark:bg-dark-900">
              <span class="flex h-9 w-9 items-center justify-center rounded-lg bg-red-600 text-sm font-bold text-white">{{ provider.initial }}</span>
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ provider.name }}</div>
                <div class="text-xs text-red-600 dark:text-red-300">{{ provider.status }}</div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-red-100 bg-white px-5 py-7 dark:border-red-900/30 dark:bg-dark-950">
      <div class="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-5">
          <router-link to="/docs" class="text-sm font-medium text-gray-500 transition-colors hover:text-red-700 dark:text-dark-400 dark:hover:text-red-300">
            {{ t('home.docs') }}
          </router-link>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="text-sm font-medium text-gray-500 transition-colors hover:text-red-700 dark:text-dark-400 dark:hover:text-red-300">
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'LightBridge')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || t('home.heroDescription'))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const githubUrl = 'https://github.com/WilliamWang1721/LightBridge'

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())

const providers = computed(() => [
  { name: t('home.providers.claude'), initial: 'C', status: t('home.providers.supported') },
  { name: 'GPT', initial: 'G', status: t('home.providers.supported') },
  { name: t('home.providers.gemini'), initial: 'G', status: t('home.providers.supported') },
  { name: t('home.providers.antigravity'), initial: 'A', status: t('home.providers.supported') },
  { name: t('home.providers.more'), initial: '+', status: t('home.providers.soon') },
])

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.code-line {
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}

.line-1 { animation-delay: 0.2s; }
.line-2 { animation-delay: 0.75s; }
.line-3 { animation-delay: 1.3s; }
.line-4 { animation-delay: 1.85s; }

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.cursor {
  display: inline-block;
  width: 8px;
  height: 18px;
  background: #fca5a5;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}
</style>
