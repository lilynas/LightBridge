<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Modules</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Installed module lifecycle and extension status.
        </p>
      </div>
      <button class="btn btn-secondary btn-sm" :disabled="loading" @click="loadModules">
        {{ loading ? 'Refreshing' : 'Refresh' }}
      </button>
    </div>

    <div class="grid gap-3 md:grid-cols-4">
      <div
        v-for="step in activationSteps"
        :key="step.title"
        class="rounded border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
      >
        <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ step.label }}</div>
        <div class="mt-2 text-sm font-semibold text-gray-900 dark:text-white">{{ step.title }}</div>
        <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ step.description }}</div>
      </div>
    </div>

    <div
      v-if="activationFocusModule"
      class="rounded border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/50 dark:bg-primary-950/20"
    >
      <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div class="min-w-0">
          <div class="text-xs font-medium uppercase tracking-wide text-primary-700 dark:text-primary-300">
            Continue module setup
          </div>
          <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">
            {{ activationFocusModule.name }}
          </div>
          <div class="mt-1 text-sm text-gray-600 dark:text-gray-300">
            {{ moduleNextStep(activationFocusModule) }}
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            class="btn btn-primary btn-sm"
            :disabled="actionLoading === activationFocusModule.id"
            @click="runActivationPrimaryAction(activationFocusModule)"
          >
            {{ activationPrimaryAction(activationFocusModule).label }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="openActivationPanel(activationFocusModule.id)">
            Details
          </button>
        </div>
      </div>
    </div>

    <div class="card overflow-hidden">
      <div class="border-b border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Marketplace</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Provider modules published by the configured registry.
            </p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ marketplaceModules.length }} available</span>
        </div>
      </div>
      <div v-if="marketplaceModules.length === 0" class="p-6 text-sm text-gray-500 dark:text-gray-400">
        No marketplace modules are available from the configured registry.
      </div>
      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Module</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Type</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Core</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Capabilities</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Permissions</th>
              <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Action</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr v-for="module in marketplaceModules" :key="`${module.id}:${module.version}`">
              <td class="px-4 py-4">
                <div class="font-medium text-gray-900 dark:text-white">{{ module.name || module.id }}</div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ module.id }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">v{{ module.version }}</div>
                <div v-if="module.description" class="mt-1 max-w-xl text-sm text-gray-500 dark:text-gray-400">
                  {{ module.description }}
                </div>
                <div v-if="module.installedStatus" class="mt-2 flex flex-wrap items-center gap-2">
                  <span class="inline-flex rounded-full px-2 py-1 text-xs font-medium" :class="statusClass(module.installedStatus)">
                    {{ module.installedStatus }}
                  </span>
                  <span v-if="module.installedVersion" class="text-xs text-gray-500 dark:text-gray-400">
                    installed v{{ module.installedVersion }}
                  </span>
                </div>
              </td>
              <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-300">{{ module.type }}</td>
              <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-300">{{ module.core }}</td>
              <td class="px-4 py-4">
                <div class="flex max-w-md flex-wrap gap-1.5">
                  <span
                    v-for="capability in module.capabilities || []"
                    :key="capability"
                    class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                  >
                    {{ capability }}
                  </span>
                  <span v-if="!module.capabilities?.length" class="text-sm text-gray-500 dark:text-gray-400">None</span>
                </div>
              </td>
              <td class="px-4 py-4">
                <div class="flex max-w-md flex-wrap gap-1.5">
                  <span
                    v-for="permission in marketplacePermissions(module)"
                    :key="permission"
                    class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                  >
                    {{ permission }}
                  </span>
                  <span v-if="marketplacePermissions(module).length === 0" class="text-sm text-gray-500 dark:text-gray-400">None</span>
                </div>
              </td>
              <td class="px-4 py-4">
                <div class="flex justify-end">
                  <button
                    class="btn btn-primary btn-xs"
                    :disabled="marketplaceInstalling === `${module.id}:${module.version}` || marketplaceInstalledCurrent(module)"
                    @click="openMarketplaceReview(module)"
                  >
                    {{ marketplaceActionLabel(module) }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="card p-4">
      <div class="flex flex-col gap-3 md:flex-row md:items-end">
        <label class="min-w-0 flex-1">
          <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">Local module package</span>
          <input
            v-model="archivePath"
            type="text"
            class="input w-full"
            placeholder="/data/module-packages/lightbridge-module-lightbridge.provider.openai-api-0.1.0.tar.zst"
            :disabled="installing"
          />
        </label>
        <button class="btn btn-primary" :disabled="installing || !archivePath.trim()" @click="installModule">
          {{ installing ? 'Installing' : 'Install' }}
        </button>
      </div>
    </div>

    <div class="card p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">Provider adapters</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Runtime-registered provider adapters from enabled modules.
          </p>
        </div>
        <span class="text-sm text-gray-500 dark:text-gray-400">{{ providerAdapters.length }} registered</span>
      </div>
      <div v-if="providerAdapters.length === 0" class="mt-4 rounded border border-dashed border-gray-200 p-4 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
        No provider adapters are currently registered.
      </div>
      <div v-else class="mt-4 flex flex-wrap gap-2">
        <span
          v-for="adapter in providerAdapters"
          :key="adapter.id"
          class="inline-flex items-center gap-2 rounded border border-green-200 bg-green-50 px-3 py-1.5 text-sm text-green-700 dark:border-green-900/50 dark:bg-green-950/30 dark:text-green-300"
        >
          <span class="h-2 w-2 rounded-full bg-green-500"></span>
          <span class="font-medium">{{ adapter.id }}</span>
          <span class="text-xs uppercase tracking-wide">{{ adapter.status }}</span>
        </span>
      </div>
    </div>

    <div class="card overflow-hidden">
      <div class="border-b border-gray-200 p-4 dark:border-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Installed modules</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Approve permissions, enable runtime, and open module-provided screens from one place.
            </p>
          </div>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ modules.length }} installed</span>
        </div>
      </div>

      <div v-if="loading" class="p-6">
        <div class="h-5 w-48 animate-pulse rounded bg-gray-200 dark:bg-dark-700"></div>
        <div class="mt-4 h-32 animate-pulse rounded bg-gray-100 dark:bg-dark-800"></div>
      </div>

      <div v-else-if="errorMessage" class="p-6">
        <p class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>
      </div>

      <div v-else-if="modules.length === 0" class="p-8 text-center">
        <p class="text-sm font-medium text-gray-700 dark:text-gray-300">No modules installed</p>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Installable modules will appear here after the registry installer is enabled.
        </p>
      </div>

      <div v-else class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Module</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Type</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Version</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Status</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Permissions</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Capabilities</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Contributions</th>
              <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr v-for="module in modules" :key="module.id">
              <td class="px-4 py-4">
                <div class="font-medium text-gray-900 dark:text-white">{{ module.name }}</div>
                <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ module.id }}</div>
                <div v-if="module.last_error" class="mt-2 rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
                  <div class="font-medium">Last error</div>
                  <div class="mt-1 break-words">{{ module.last_error }}</div>
                  <div class="mt-1 text-red-600 dark:text-red-300">{{ moduleHealthHint(module) }}</div>
                </div>
              </td>
              <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-300">{{ module.type }}</td>
              <td class="px-4 py-4 text-sm text-gray-700 dark:text-gray-300">{{ module.version }}</td>
              <td class="px-4 py-4">
                <span class="inline-flex rounded-full px-2 py-1 text-xs font-medium" :class="statusClass(module.status)">
                  {{ module.status }}
                </span>
                <div class="mt-2 max-w-48 text-xs text-gray-500 dark:text-gray-400">
                  {{ moduleNextStep(module) }}
                </div>
              </td>
              <td class="px-4 py-4">
                <div v-if="permissionStatus(module.id)?.permissions.length" class="max-w-md space-y-2">
                  <span
                    class="inline-flex rounded-full px-2 py-1 text-xs font-medium"
                    :class="permissionStatus(module.id)?.approved ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'"
                  >
                    {{ permissionStatus(module.id)?.approved ? 'approved' : 'approval required' }}
                  </span>
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="permission in permissionStatus(module.id)?.permissions"
                      :key="`${permission.permission_type}:${permission.permission_value}`"
                      class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                    >
                      {{ permission.permission_type }}: {{ permission.permission_value }}
                    </span>
                  </div>
                </div>
                <span v-else class="text-sm text-gray-500 dark:text-gray-400">None</span>
              </td>
              <td class="px-4 py-4">
                <div class="flex max-w-md flex-wrap gap-1.5">
                  <span
                    v-for="capability in module.manifest.capabilities"
                    :key="capability"
                    class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                  >
                    {{ capability }}
                  </span>
                </div>
              </td>
              <td class="px-4 py-4">
                <div v-if="moduleContributions(module).length" class="max-w-md space-y-2">
                  <div
                    v-for="contribution in moduleContributions(module)"
                    :key="`${contribution.type}:${contribution.label}`"
                    class="rounded border border-gray-200 bg-gray-50 p-2 text-xs dark:border-dark-700 dark:bg-dark-800"
                  >
                    <div class="flex items-center justify-between gap-2">
                      <span class="font-medium text-gray-800 dark:text-gray-100">{{ contribution.type }}</span>
                      <RouterLink
                        v-if="contribution.path"
                        :to="contribution.path"
                        class="text-primary-600 hover:text-primary-700 dark:text-primary-300"
                      >
                        Open
                      </RouterLink>
                      <RouterLink
                        v-else-if="contribution.accountProviderId"
                        :to="createAccountRoute(contribution.accountProviderId)"
                        class="text-primary-600 hover:text-primary-700 dark:text-primary-300"
                      >
                        Create account
                      </RouterLink>
                    </div>
                    <div class="mt-1 break-all text-gray-500 dark:text-gray-400">{{ contribution.label }}</div>
                  </div>
                </div>
                <span v-else class="text-sm text-gray-500 dark:text-gray-400">None</span>
              </td>
              <td class="px-4 py-4">
                <div class="flex justify-end gap-2">
                  <button
                    v-if="module.status === 'enabled' && primaryModuleUse(module)"
                    class="btn btn-primary btn-xs"
                    :disabled="actionLoading === module.id"
                    @click="openModuleUse(module)"
                  >
                    {{ primaryModuleUse(module)?.label }}
                  </button>
                  <button
                    class="btn btn-secondary btn-xs"
                    :disabled="actionLoading === module.id"
                    @click="openActivationPanel(module.id)"
                  >
                    Continue
                  </button>
                  <button
                    v-if="permissionStatus(module.id) && !permissionStatus(module.id)?.approved"
                    class="btn btn-secondary btn-xs"
                    :disabled="actionLoading === module.id"
                    @click="approveModulePermissions(module.id)"
                  >
                    Approve
                  </button>
                  <button
                    v-if="module.status !== 'enabled'"
                    class="btn btn-primary btn-xs"
                    :disabled="actionLoading === module.id || permissionStatus(module.id)?.approved === false"
                    @click="runAction(module.id, 'enable')"
                  >
                    Enable
                  </button>
                  <div
                    v-if="module.status !== 'enabled' && permissionStatus(module.id)?.approved === false"
                    class="w-full text-right text-xs text-amber-600 dark:text-amber-300"
                  >
                    Approve first
                  </div>
                  <button
                    v-if="module.status === 'enabled'"
                    class="btn btn-secondary btn-xs"
                    :disabled="actionLoading === module.id"
                    @click="runAction(module.id, 'disable')"
                  >
                    Disable
                  </button>
                  <button
                    class="btn btn-secondary btn-xs"
                    :disabled="actionLoading === module.id || module.status === 'purged'"
                    @click="runAction(module.id, 'uninstall')"
                  >
                    Uninstall
                  </button>
                  <button
                    class="btn btn-xs border border-red-200 bg-red-50 text-red-700 hover:bg-red-100 disabled:opacity-50 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300 dark:hover:bg-red-900/40"
                    :disabled="actionLoading === module.id || module.status === 'purged'"
                    @click="runAction(module.id, 'purge')"
                  >
                    Purge
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div
      v-if="activeActivationModule"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      role="dialog"
      aria-modal="true"
    >
      <div class="w-full max-w-3xl rounded-lg bg-white shadow-xl dark:bg-dark-900">
        <div class="border-b border-gray-200 p-5 dark:border-dark-700">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Module activation
              </div>
              <h2 class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">
                {{ activeActivationModule.name }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ moduleNextStep(activeActivationModule) }}
              </p>
            </div>
            <button class="btn btn-secondary btn-sm" :disabled="Boolean(actionLoading)" @click="closeActivationPanel">
              Close
            </button>
          </div>
        </div>

        <div class="space-y-5 p-5">
          <div class="grid gap-3 md:grid-cols-4">
            <div
              v-for="step in activationProgress(activeActivationModule)"
              :key="step.title"
              class="rounded border p-3"
              :class="step.done ? 'border-green-200 bg-green-50 dark:border-green-900/50 dark:bg-green-950/30' : step.current ? 'border-blue-200 bg-blue-50 dark:border-blue-900/50 dark:bg-blue-950/30' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900'"
            >
              <div
                class="inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
                :class="step.done ? 'bg-green-600 text-white' : step.current ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300'"
              >
                {{ step.done ? 'OK' : step.index }}
              </div>
              <div class="mt-2 text-sm font-semibold text-gray-900 dark:text-white">{{ step.title }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ step.description }}</div>
            </div>
          </div>

          <div class="rounded border border-gray-200 p-4 dark:border-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div class="text-sm font-semibold text-gray-900 dark:text-white">Next action</div>
                <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ activationPrimaryAction(activeActivationModule).description }}
                </div>
              </div>
              <button
                v-if="activationPrimaryAction(activeActivationModule).kind === 'approve'"
                class="btn btn-primary"
                :disabled="actionLoading === activeActivationModule.id"
                @click="approveModulePermissions(activeActivationModule.id)"
              >
                Approve permissions
              </button>
              <button
                v-else-if="activationPrimaryAction(activeActivationModule).kind === 'enable'"
                class="btn btn-primary"
                :disabled="actionLoading === activeActivationModule.id"
                @click="runAction(activeActivationModule.id, 'enable')"
              >
                Enable module
              </button>
              <button
                v-else-if="activationPrimaryAction(activeActivationModule).kind === 'open'"
                class="btn btn-primary"
                @click="openModuleUse(activeActivationModule)"
              >
                {{ activationPrimaryAction(activeActivationModule).label }}
              </button>
              <button v-else class="btn btn-secondary" @click="closeActivationPanel">
                Done
              </button>
            </div>
          </div>

          <div v-if="moduleContributions(activeActivationModule).length" class="grid gap-3 md:grid-cols-2">
            <div
              v-for="contribution in moduleContributions(activeActivationModule)"
              :key="`activation:${contribution.type}:${contribution.label}`"
              class="rounded border border-gray-200 p-3 dark:border-dark-700"
            >
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ contribution.type }}</div>
              <div class="mt-1 break-all text-sm text-gray-900 dark:text-white">{{ contribution.label }}</div>
              <div class="mt-3">
                <RouterLink
                  v-if="contribution.path"
                  :to="contribution.path"
                  class="btn btn-secondary btn-sm"
                >
                  Open
                </RouterLink>
                <RouterLink
                  v-else-if="contribution.accountProviderId"
                  :to="createAccountRoute(contribution.accountProviderId)"
                  class="btn btn-secondary btn-sm"
                >
                  Create account
                </RouterLink>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="reviewingMarketplaceModule"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      role="dialog"
      aria-modal="true"
    >
      <div class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-lg bg-white shadow-xl dark:bg-dark-900">
        <div class="border-b border-gray-200 p-5 dark:border-dark-700">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Review module {{ reviewingMarketplaceActionLabel() }}
              </div>
              <h2 class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">
                {{ reviewingMarketplaceModule.name || reviewingMarketplaceModule.id }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Confirm source, permissions, and extension surface before changing this module version.
              </p>
            </div>
            <button class="btn btn-secondary btn-sm" :disabled="Boolean(marketplaceInstalling)" @click="closeMarketplaceReview">
              Close
            </button>
          </div>
        </div>

        <div class="space-y-5 p-5">
          <div class="grid gap-3 md:grid-cols-2">
            <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Module ID</div>
              <div class="mt-1 break-all text-sm text-gray-900 dark:text-white">{{ reviewingMarketplaceModule.id }}</div>
            </div>
            <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Version</div>
              <div class="mt-1 text-sm text-gray-900 dark:text-white">
                {{ reviewingMarketplaceModule.version }}
                <span v-if="reviewingMarketplaceModule.installedVersion" class="text-gray-500 dark:text-gray-400">
                  from {{ reviewingMarketplaceModule.installedVersion }}
                </span>
              </div>
            </div>
            <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Core compatibility</div>
              <div class="mt-1 text-sm text-gray-900 dark:text-white">{{ reviewingMarketplaceModule.core }}</div>
            </div>
            <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Package verification</div>
              <div class="mt-1 text-sm text-gray-900 dark:text-white">
                {{ reviewingMarketplaceModule.sha256 ? 'Registry SHA256 provided' : 'Package signature only' }}
              </div>
            </div>
          </div>

          <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Download source</div>
            <div class="mt-1 break-all text-sm text-gray-700 dark:text-gray-300">{{ reviewingMarketplaceModule.downloadUrl }}</div>
            <div v-if="reviewingMarketplaceModule.sha256" class="mt-2 break-all text-xs text-gray-500 dark:text-gray-400">
              SHA256 {{ reviewingMarketplaceModule.sha256 }}
            </div>
          </div>

          <div class="grid gap-3 md:grid-cols-2">
            <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Capabilities</div>
              <div class="mt-2 flex flex-wrap gap-1.5">
                <span
                  v-for="capability in reviewingMarketplaceModule.capabilities || []"
                  :key="capability"
                  class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                >
                  {{ capability }}
                </span>
                <span v-if="!reviewingMarketplaceModule.capabilities?.length" class="text-sm text-gray-500 dark:text-gray-400">None</span>
              </div>
            </div>
            <div class="rounded border border-gray-200 p-3 dark:border-dark-700">
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">Requested permissions</div>
              <div class="mt-2 flex flex-wrap gap-1.5">
                <span
                  v-for="permission in reviewingMarketplacePermissions()"
                  :key="permission"
                  class="rounded bg-amber-50 px-2 py-1 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300"
                >
                  {{ permission }}
                </span>
                <span v-if="reviewingMarketplacePermissions().length === 0" class="text-sm text-gray-500 dark:text-gray-400">None</span>
              </div>
            </div>
          </div>

          <div class="rounded border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200">
            After installation, permissions must be approved before enabling runtime and UI contributions.
          </div>
        </div>

        <div class="flex justify-end gap-2 border-t border-gray-200 p-5 dark:border-dark-700">
          <button class="btn btn-secondary" :disabled="Boolean(marketplaceInstalling)" @click="closeMarketplaceReview">
            Cancel
          </button>
          <button
            class="btn btn-primary"
            :disabled="Boolean(marketplaceInstalling)"
            @click="confirmMarketplaceReview"
          >
            {{ marketplaceInstalling ? 'Working' : reviewingMarketplaceActionLabel() }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import modulesAPI, {
  type InstalledModule,
  type MarketplaceModule,
  type ModulePermissionStatus,
  type ModuleProviderAdapterStatus,
  type ModuleStatus
} from '@/api/admin/modules'

type ModuleAction = 'enable' | 'disable' | 'uninstall' | 'purge'
type MarketplaceVersionAction = 'install' | 'upgrade' | 'rollback' | 'current'

interface ModuleContributionSummary {
  type: string
  label: string
  path?: string
  accountProviderId?: string
}

interface ModuleActivationStep {
  index: number
  title: string
  description: string
  done: boolean
  current: boolean
}

interface ModuleUseAction {
  label: string
  route: RouteLocationRaw
}

type ActivationAction =
  | { kind: 'approve'; description: string; label: string }
  | { kind: 'enable'; description: string; label: string }
  | { kind: 'open'; description: string; label: string }
  | { kind: 'done'; description: string; label: string }

const router = useRouter()

const modules = ref<InstalledModule[]>([])
const marketplaceModules = ref<MarketplaceModule[]>([])
const providerAdapters = ref<ModuleProviderAdapterStatus[]>([])
const permissionsByModule = ref<Record<string, ModulePermissionStatus>>({})
const reviewingMarketplaceModule = ref<MarketplaceModule | null>(null)
const loading = ref(false)
const actionLoading = ref('')
const installing = ref(false)
const marketplaceInstalling = ref('')
const archivePath = ref('')
const errorMessage = ref('')
const activeActivationModuleId = ref('')

const activeActivationModule = computed(() => {
  return modules.value.find((module) => module.id === activeActivationModuleId.value) || null
})

const activationFocusModule = computed(() => {
  const candidates = modules.value.filter((module) => !['purged', 'uninstalled'].includes(module.status))
  return candidates.find((module) => {
    const permission = permissionStatus(module.id)
    return permission && !permission.approved
  }) ||
    candidates.find((module) => module.status !== 'enabled') ||
    candidates.find((module) => module.status === 'enabled' && Boolean(primaryModuleUse(module))) ||
    null
})

const activationSteps = [
  {
    label: 'Step 1',
    title: 'Review',
    description: 'Check source, version, permissions, and package verification before install.'
  },
  {
    label: 'Step 2',
    title: 'Install',
    description: 'Core verifies package signature, checksums, manifest, migrations, and files.'
  },
  {
    label: 'Step 3',
    title: 'Approve',
    description: 'Approve declared permissions for this installed module version.'
  },
  {
    label: 'Step 4',
    title: 'Enable',
    description: 'Start runtime contributions and expose routes, menus, and account forms.'
  }
]

onMounted(loadModules)

async function loadModules() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [installedModules, adapters, marketplace] = await Promise.all([
      modulesAPI.listInstalled(),
      modulesAPI.listProviderAdapters(),
      modulesAPI.listMarketplace()
    ])
    modules.value = installedModules
    providerAdapters.value = adapters
    marketplaceModules.value = marketplace.modules
    await loadPermissionStatuses(installedModules)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to load modules'
  } finally {
    loading.value = false
  }
}

async function loadPermissionStatuses(items = modules.value) {
  const entries = await Promise.all(items.map(async (item) => {
    try {
      return [item.id, await modulesAPI.permissions(item.id)] as const
    } catch {
      return [item.id, { permissions: [], approved: true }] as const
    }
  }))
  permissionsByModule.value = Object.fromEntries(entries)
}

function permissionStatus(id: string): ModulePermissionStatus | undefined {
  return permissionsByModule.value[id]
}

async function runAction(id: string, action: ModuleAction) {
  if (action === 'purge') {
    const confirmed = window.confirm('Purge this module and delete its private data? This action cannot be undone.')
    if (!confirmed) {
      return
    }
  }
  actionLoading.value = id
  errorMessage.value = ''
  try {
    if (action === 'purge') {
      await modulesAPI.purge(id, true)
      if (activeActivationModuleId.value === id) {
        activeActivationModuleId.value = ''
      }
    } else {
      await modulesAPI[action](id)
    }
    await refreshModuleState()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : `Failed to ${action} module`
  } finally {
    actionLoading.value = ''
  }
}

async function approveModulePermissions(id: string) {
  actionLoading.value = id
  errorMessage.value = ''
  try {
    permissionsByModule.value[id] = await modulesAPI.approvePermissions(id)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to approve module permissions'
  } finally {
    actionLoading.value = ''
  }
}

async function installModule() {
  installing.value = true
  errorMessage.value = ''
  try {
    const archive = archivePath.value.trim()
    const installedModule = await modulesAPI.installArchive(archive)
    await refreshModuleState()
    const installed = modules.value.find((module) => module.id === installedModule.id) || installedModule
    activeActivationModuleId.value = installed.id
    archivePath.value = ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to install module package'
  } finally {
    installing.value = false
  }
}

function openMarketplaceReview(module: MarketplaceModule) {
  if (marketplaceInstalledCurrent(module)) {
    return
  }
  reviewingMarketplaceModule.value = module
}

function closeMarketplaceReview() {
  if (marketplaceInstalling.value) {
    return
  }
  reviewingMarketplaceModule.value = null
}

async function confirmMarketplaceReview() {
  if (!reviewingMarketplaceModule.value) {
    return
  }
  await changeMarketplaceModuleVersion(reviewingMarketplaceModule.value)
}

function reviewingMarketplaceActionLabel(): string {
  return reviewingMarketplaceModule.value ? marketplaceActionLabel(reviewingMarketplaceModule.value) : ''
}

function reviewingMarketplacePermissions(): string[] {
  return reviewingMarketplaceModule.value ? marketplacePermissions(reviewingMarketplaceModule.value) : []
}

async function changeMarketplaceModuleVersion(module: MarketplaceModule) {
  const key = `${module.id}:${module.version}`
  marketplaceInstalling.value = key
  errorMessage.value = ''
  try {
    const action = marketplaceVersionAction(module)
    if (action === 'upgrade') {
      await modulesAPI.upgrade(module.id, module.version)
    } else if (action === 'rollback') {
      await modulesAPI.rollback(module.id, module.version)
    } else {
      await modulesAPI.installFromMarketplace(module.id, module.version)
    }
    await refreshModuleState()
    reviewingMarketplaceModule.value = null
    activeActivationModuleId.value = module.id
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Failed to change module version'
  } finally {
    marketplaceInstalling.value = ''
  }
}

async function refreshModuleState() {
  const [installedModules, adapters, marketplace] = await Promise.all([
    modulesAPI.listInstalled(),
    modulesAPI.listProviderAdapters(),
    modulesAPI.listMarketplace()
  ])
  modules.value = installedModules
  providerAdapters.value = adapters
  marketplaceModules.value = marketplace.modules
  await loadPermissionStatuses(installedModules)
}

function marketplacePermissions(module: MarketplaceModule): string[] {
  if (!module.permissions) {
    return []
  }
  return Object.entries(module.permissions).flatMap(([type, values]) =>
    values.map((value) => `${type}: ${value}`)
  )
}

function marketplaceInstalledCurrent(module: MarketplaceModule): boolean {
  return Boolean(
    module.installedVersion === module.version &&
    module.installedStatus &&
    !['uninstalled', 'purged'].includes(module.installedStatus)
  )
}

function marketplaceVersionAction(module: MarketplaceModule): MarketplaceVersionAction {
  if (marketplaceInstalledCurrent(module)) {
    return 'current'
  }
  if (!module.installedVersion) {
    return 'install'
  }
  const comparison = compareSemver(module.version, module.installedVersion)
  if (comparison > 0) {
    return 'upgrade'
  }
  if (comparison < 0) {
    return 'rollback'
  }
  return 'install'
}

function marketplaceActionLabel(module: MarketplaceModule): string {
  const key = `${module.id}:${module.version}`
  if (marketplaceInstalling.value === key) {
    return 'Working'
  }
  switch (marketplaceVersionAction(module)) {
    case 'current':
      return 'Installed'
    case 'upgrade':
      return 'Upgrade'
    case 'rollback':
      return 'Rollback'
    default:
      break
  }
  if (module.installedStatus === 'uninstalled' || module.installedStatus === 'purged') {
    return 'Reinstall'
  }
  return 'Install'
}

function moduleContributions(module: InstalledModule): ModuleContributionSummary[] {
  const frontend = module.manifest.frontend
  if (!frontend) {
    return []
  }
  const routes = (frontend.routes || []).map((route) => ({
    type: 'Admin route',
    label: `${route.title} · ${route.path}`,
    path: route.path
  }))
  const menus = (frontend.menu || []).map((menu) => ({
    type: 'Sidebar menu',
    label: `${menu.title} · ${menu.path}`,
    path: menu.path
  }))
  const accountForms = (frontend.accountForms || []).map((form) => ({
    type: 'Account form',
    label: `${form.providerName || form.providerId} · ${form.exposedModule}`,
    accountProviderId: form.providerId
  }))
  return [...routes, ...menus, ...accountForms]
}

function createAccountRoute(providerId: string): RouteLocationRaw {
  return {
    name: 'AdminAccounts',
    query: {
      create: 'module-account',
      module_provider_id: providerId
    }
  }
}

function primaryModuleUse(module: InstalledModule): ModuleUseAction | null {
  if (module.status !== 'enabled') {
    return null
  }
  const contributions = moduleContributions(module)
  const accountForm = contributions.find((contribution) => contribution.accountProviderId)
  if (accountForm?.accountProviderId) {
    return {
      label: 'Create account',
      route: createAccountRoute(accountForm.accountProviderId)
    }
  }
  const routeContribution = contributions.find((contribution) => contribution.path)
  if (routeContribution?.path) {
    return {
      label: 'Open module',
      route: routeContribution.path
    }
  }
  return null
}

function openModuleUse(module: InstalledModule) {
  const action = primaryModuleUse(module)
  if (!action) {
    return
  }
  void router.push(action.route)
}

function openActivationPanel(id: string) {
  activeActivationModuleId.value = id
}

function closeActivationPanel() {
  activeActivationModuleId.value = ''
}

function activationProgress(module: InstalledModule): ModuleActivationStep[] {
  const permission = permissionStatus(module.id)
  const installed = !['uninstalled', 'purged'].includes(module.status)
  const approved = !permission || permission.approved
  const enabled = module.status === 'enabled'
  const usable = enabled && Boolean(primaryModuleUse(module))
  const states = [
    {
      title: 'Installed',
      description: 'Package is available in Core.',
      done: installed
    },
    {
      title: 'Approved',
      description: permission && !permission.approved ? 'Permission approval is required.' : 'Permissions are ready.',
      done: installed && approved
    },
    {
      title: 'Enabled',
      description: 'Runtime and UI contributions are active.',
      done: enabled
    },
    {
      title: 'Use',
      description: usable ? 'Open the module or create a module account.' : 'Enable the module to expose actions.',
      done: usable
    }
  ]
  const currentIndex = states.findIndex((step) => !step.done)
  return states.map((step, index) => ({
    index: index + 1,
    title: step.title,
    description: step.description,
    done: step.done,
    current: currentIndex === index
  }))
}

function activationPrimaryAction(module: InstalledModule): ActivationAction {
  const permission = permissionStatus(module.id)
  const useAction = primaryModuleUse(module)
  if (permission && !permission.approved) {
    return {
      kind: 'approve',
      label: 'Approve permissions',
      description: 'Approve the permissions requested by this module version before enabling it.'
    }
  }
  if (module.status !== 'enabled') {
    return {
      kind: 'enable',
      label: 'Enable module',
      description: 'Enable the module to register provider adapters, menu items, routes, and account forms.'
    }
  }
  if (useAction) {
    return {
      kind: 'open',
      label: useAction.label,
      description: 'The module is ready. Continue directly into the module workflow.'
    }
  }
  return {
    kind: 'done',
    label: 'Done',
    description: 'The module runtime is enabled. No user-facing contribution is declared for this module.'
  }
}

function runActivationPrimaryAction(module: InstalledModule) {
  const action = activationPrimaryAction(module)
  if (action.kind === 'approve') {
    void approveModulePermissions(module.id)
    return
  }
  if (action.kind === 'enable') {
    void runAction(module.id, 'enable')
    return
  }
  if (action.kind === 'open') {
    openModuleUse(module)
    return
  }
  openActivationPanel(module.id)
}

function moduleHealthHint(module: InstalledModule): string {
  const error = module.last_error?.toLowerCase() || ''
  if (error.includes('permission')) {
    return 'Review and approve permissions, then enable the module again.'
  }
  if (error.includes('checksum') || error.includes('signature') || error.includes('verify')) {
    return 'Reinstall from a trusted package source before enabling.'
  }
  if (error.includes('runtime') || error.includes('sidecar') || error.includes('socket')) {
    return 'Inspect the sidecar binary, executable bit, socket path, and runtime logs.'
  }
  if (module.status === 'failed') {
    return 'Disable or reinstall the module after resolving the reported failure.'
  }
  return 'Resolve the reported issue, then refresh module state.'
}

function moduleNextStep(module: InstalledModule): string {
  const permission = permissionStatus(module.id)
  if (module.status === 'enabled') {
    const contributions = moduleContributions(module)
    return contributions.length > 0 ? 'Ready. Open the module contribution below.' : 'Ready. Runtime contributions are active.'
  }
  if (permission && !permission.approved) {
    return 'Approve requested permissions before enabling.'
  }
  if (module.status === 'installed' || module.status === 'disabled') {
    return 'Enable to register runtime and UI contributions.'
  }
  if (module.status === 'failed') {
    return 'Fix the reported failure, then reinstall or enable again.'
  }
  if (module.status === 'uninstalled') {
    return 'Reinstall from marketplace or local package.'
  }
  if (module.status === 'purged') {
    return 'Private module data has been removed.'
  }
  return 'Review module status before continuing.'
}

function compareSemver(left: string, right: string): number {
  const a = parseSemver(left)
  const b = parseSemver(right)
  for (let index = 0; index < 3; index += 1) {
    if (a[index] !== b[index]) {
      return a[index] - b[index]
    }
  }
  return 0
}

function parseSemver(value: string): [number, number, number] {
  const normalized = value.trim().replace(/^v/, '').split(/[+-]/)[0]
  const [major = '0', minor = '0', patch = '0'] = normalized.split('.')
  return [Number(major) || 0, Number(minor) || 0, Number(patch) || 0]
}

function statusClass(status: ModuleStatus): string {
  switch (status) {
    case 'enabled':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    case 'failed':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'disabled':
    case 'uninstalled':
      return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
    case 'purged':
      return 'bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300'
    default:
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  }
}
</script>
