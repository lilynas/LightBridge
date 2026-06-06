export const MockProviderSettings = {
  name: 'MockProviderSettings',
  props: {
    moduleId: { type: String, required: true },
    moduleName: { type: String, required: true }
  },
  template: `
    <section class="space-y-4">
      <div class="rounded border border-green-200 bg-green-50 p-4 text-green-800">
        <h2 class="text-base font-semibold">Mock provider runtime is enabled</h2>
        <p class="mt-1 text-sm">
          This module contributes the admin route, sidebar item, account form, and provider adapter.
        </p>
      </div>
      <dl class="grid gap-3 text-sm sm:grid-cols-2">
        <div class="rounded border border-gray-200 p-3">
          <dt class="text-xs font-medium uppercase text-gray-500">Module ID</dt>
          <dd class="mt-1 break-all text-gray-900">{{ moduleId }}</dd>
        </div>
        <div class="rounded border border-gray-200 p-3">
          <dt class="text-xs font-medium uppercase text-gray-500">Provider model</dt>
          <dd class="mt-1 text-gray-900">mock-stream</dd>
        </div>
      </dl>
    </section>
  `
}

export const MockAccountForm = {
  name: 'MockAccountForm',
  emits: ['submit', 'cancel'],
  data() {
    return {
      displayName: 'Mock Provider Account',
      apiKey: 'mock-local-key'
    }
  },
  methods: {
    submitForm() {
      this.$emit('submit', {
        name: this.displayName,
        credential_type: 'api_key',
        credentials: {
          mock_api_key: this.apiKey
        },
        module_config: {
          model: 'mock-stream'
        },
        extra: {
          display_name: this.displayName
        }
      })
    }
  },
  template: `
    <div class="space-y-4">
      <label class="block">
        <span class="mb-1 block text-sm font-medium text-gray-700">Display name</span>
        <input v-model="displayName" class="input w-full" type="text" required />
      </label>
      <label class="block">
        <span class="mb-1 block text-sm font-medium text-gray-700">Mock API key</span>
        <input v-model="apiKey" class="input w-full" type="text" required />
      </label>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" type="button" @click="$emit('cancel')">Cancel</button>
        <button class="btn btn-primary" type="button" @click="submitForm">Save mock account</button>
      </div>
    </div>
  `
}

export default MockProviderSettings
