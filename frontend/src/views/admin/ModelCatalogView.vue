<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <ModelCatalogPanel
        admin
        :models="models"
        :loading="loading"
        @refresh="loadCatalog"
        @quickSetup="openQuickSetup"
      />
    </div>

    <QuickMonitorSetupDialog
      :visible="showQuickSetup"
      :model-id="setupModel?.id || ''"
      :platform="setupModel?.platform"
      @close="showQuickSetup = false"
      @created="loadCatalog"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelCatalogPanel from '@/components/model-catalog/ModelCatalogPanel.vue'
import QuickMonitorSetupDialog from '@/components/model-catalog/QuickMonitorSetupDialog.vue'
import { getAdminModelCatalog, type ModelCatalogModel } from '@/api/modelCatalog'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const models = ref<ModelCatalogModel[]>([])
const loading = ref(false)
const showQuickSetup = ref(false)
const setupModel = ref<ModelCatalogModel | null>(null)

function openQuickSetup(model: ModelCatalogModel) {
  setupModel.value = model
  showQuickSetup.value = true
}

async function loadCatalog() {
  loading.value = true
  try {
    const data = await getAdminModelCatalog()
    models.value = data.models || []
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadCatalog)
</script>
