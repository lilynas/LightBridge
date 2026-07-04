<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <ModelCatalogPanel
        :models="models"
        :loading="loading"
        @refresh="loadCatalog"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelCatalogPanel from '@/components/model-catalog/ModelCatalogPanel.vue'
import { getUserModelCatalog, type ModelCatalogModel } from '@/api/modelCatalog'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const appStore = useAppStore()

const models = ref<ModelCatalogModel[]>([])
const loading = ref(false)

async function loadCatalog() {
  loading.value = true
  try {
    const data = await getUserModelCatalog()
    models.value = data.models || []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadCatalog)
</script>
