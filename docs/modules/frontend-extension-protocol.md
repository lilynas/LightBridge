# Frontend Extension Protocol

## UI Manifest Endpoint

Core exposes enabled frontend contributions at:

```http
GET /api/v1/modules/ui-manifest
```

Response shape:

```json
[
  {
    "moduleId": "lightbridge.provider.openai-api",
    "version": "0.1.0",
    "remoteEntry": "/modules/lightbridge.provider.openai-api/0.1.0/frontend/remoteEntry.js",
    "routes": [
      {
        "path": "/admin/providers/openai",
        "title": "OpenAI API",
        "exposedModule": "./OpenAIProviderSettings",
        "requiresAdmin": true
      }
    ],
    "menu": [
      {
        "title": "OpenAI API",
        "path": "/admin/providers/openai",
        "group": "Providers",
        "order": 100
      }
    ],
    "accountForms": [
      {
        "providerId": "lightbridge.provider.openai-api",
        "exposedModule": "./OpenAIAccountForm"
      }
    ]
  }
]
```

## Type Contract

The shell should treat the manifest as data from an untrusted module package.
Validate every field before registering routes.

```ts
export interface ModuleUIManifestEntry {
  moduleId: string
  version: string
  remoteEntry: string
  routes: ModuleRouteContribution[]
  menu: ModuleMenuContribution[]
  accountForms: ModuleAccountFormContribution[]
}

export interface ModuleRouteContribution {
  path: string
  title: string
  exposedModule: string
  requiresAdmin?: boolean
}

export interface ModuleMenuContribution {
  title: string
  path: string
  group: string
  order?: number
}

export interface ModuleAccountFormContribution {
  providerId: string
  exposedModule: string
}
```

Validation rules:

- `remoteEntry` must be served by Core from the installed module asset path or
  another explicitly trusted module asset origin.
- `routes[].path` must start with `/admin/` for admin routes.
- `routes[].exposedModule` and `accountForms[].exposedModule` must start with
  `./`.
- `accountForms[].providerId` must equal the entry `moduleId` in the provider
  MVP. Core then enables the provider adapter under that same ID.
- duplicate route paths are rejected; duplicate menu items are ignored after
  the first stable sort.

## Loading Rules

- Load the UI manifest after public settings and auth state are ready.
- Register routes with `router.addRoute`.
- Merge module menu items into existing sidebar groups.
- If a remote fails to load, show a module error page for that route and keep the core shell usable.
- Cache-bust remote entries by module version.

Recommended shell flow:

```ts
export async function installModuleUI(router: Router, menuStore: MenuStore) {
  const manifest = await api.get<ModuleUIManifestEntry[]>('/api/v1/modules/ui-manifest')
  const validEntries = validateModuleManifest(manifest)

  for (const entry of validEntries) {
    for (const route of entry.routes) {
      router.addRoute('AdminShell', {
        path: route.path,
        name: `module:${entry.moduleId}:${route.path}`,
        meta: {
          title: route.title,
          requiresAdmin: route.requiresAdmin !== false,
          moduleId: entry.moduleId,
          moduleVersion: entry.version,
        },
        component: () => loadModuleRoute(entry, route),
      })
    }
    menuStore.addModuleItems(entry.moduleId, entry.menu)
  }
}

async function loadModuleRoute(entry: ModuleUIManifestEntry, route: ModuleRouteContribution) {
  try {
    const remote = await loadRemoteEntry(`${entry.remoteEntry}?v=${entry.version}`)
    return await remote.get(route.exposedModule)
  } catch (error) {
    return ModuleRemoteErrorPage.withProps({
      moduleId: entry.moduleId,
      version: entry.version,
      routePath: route.path,
      error,
    })
  }
}
```

The remote loader implementation may use Vite remote ESM or
`vite-plugin-federation`, but the shell-level behavior above is required.

## Shared Dependencies

Frontend modules may rely only on the public extension SDK:

- Vue
- Vue Router
- Pinia
- i18n helper
- API client wrapper
- design tokens and base components explicitly exported by the SDK

Modules must not import core internal business components by path such as `@/components/account/*`.

The public extension SDK is the only import surface that modules can rely on:

```ts
export interface LightBridgeExtensionSDK {
  vue: typeof import('vue')
  router: Router
  pinia: Pinia
  i18n: I18n
  api: {
    get<T>(path: string, options?: RequestOptions): Promise<T>
    post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>
    put<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>
    delete<T>(path: string, options?: RequestOptions): Promise<T>
  }
  notify: {
    success(message: string): void
    error(message: string): void
    warning(message: string): void
  }
  tokens: {
    colors: Record<string, string>
    spacing: Record<string, string>
  }
}
```

The SDK must not expose Core repositories, auth tokens, raw stores with
unrelated data, or internal account/provider components.

## Remote Component Contract

Each route `exposedModule` must default-export a Vue component:

```ts
export default defineComponent({
  name: 'MockProviderSettings',
  setup() {
    const sdk = inject<LightBridgeExtensionSDK>('lightbridge.extension')
    return {}
  },
})
```

Each account form contribution must default-export a Vue component that emits a
validated submit payload:

```ts
export interface ProviderAccountFormSubmit {
  credential_type: string
  credentials: Record<string, unknown>
  module_config?: Record<string, unknown>
  extra?: Record<string, unknown>
}
```

Account form component contract:

```ts
defineEmits<{
  submit: [payload: ProviderAccountFormSubmit]
  cancel: []
}>()
```

The shell wraps the payload into the shared account API request:

```json
{
  "platform": "module",
  "type": "module",
  "provider_id": "lightbridge.provider.mock",
  "credentials": {},
  "extra": {
    "provider_id": "lightbridge.provider.mock",
    "module_id": "lightbridge.provider.mock"
  }
}
```

The module form should validate provider-specific fields, but Core remains the
authority for permission checks, secret storage, account IDs, and provider
registration.

The shell must not invent provider aliases. For provider-module MVP accounts,
the `provider_id`, `extra.provider_id`, `extra.module_id`, UI manifest
`moduleId`, and account-form `providerId` are the same string.

## Failure Behavior

Remote failures are isolated to the contributed route or form.

| Failure | Required Shell Behavior |
| --- | --- |
| UI manifest fetch fails | Keep Core shell usable; show an admin-visible module UI warning. |
| Manifest entry invalid | Skip the invalid module UI contribution; keep the module backend state unchanged. |
| `remoteEntry.js` 404 | Route renders module error page; sidebar item may stay visible with error state. |
| Exposed route module missing | Route renders module error page. |
| Account form remote fails | Show provider account form error and keep provider selection page usable. |
| Shared dependency missing | Route renders module error page and logs module ID/version. |

Module error pages must show:

- module ID
- module version
- failed route or contribution
- retry action
- disable-module action for admins when the API is available

They must not show raw stack traces to non-admin users.

## Remote Assets And CSP

Module frontend assets are served by Core from the installed package directory.
The shell must not load arbitrary remote JavaScript from user-provided URLs in
the MVP.

Allowed asset sources:

| Source | Allowed | Notes |
| --- | --- | --- |
| `/modules/<module-id>/<version>/frontend/remoteEntry.js` | yes | Preferred installed package asset path. |
| `/modules/<module-id>/<version>/frontend/assets/*` | yes | Static assets referenced by the remote. |
| `http://` or `https://` third-party script URL | no for MVP | Use signed package assets instead. |
| Core internal source path such as `@/views/...` | no | Modules must use public SDK only. |

The admin shell should apply these rules:

- append the module version to remote entry requests for cache busting
- do not share raw auth tokens with module remotes
- use the Core API client wrapper from the public SDK
- log module ID/version when remote loading fails
- keep remote asset failures isolated from the Core shell

If a future CSP policy is added, it must allow installed module asset paths and
deny arbitrary script origins by default. Updating CSP for a new module source is
a module runtime/platform change and must be documented here before
implementation.

## Local UI Smoke Test

Use this smoke test before calling a frontend contribution complete:

1. Start Core with a signed mock provider module enabled.
2. Open `/api/v1/modules/ui-manifest`.
3. Confirm the mock provider has one route, one menu item, and one account form
   contribution.
4. Navigate to the contributed route.
5. Confirm the route component loads from `remoteEntry.js`.
6. Break the `remoteEntry.js` path and reload.
7. Confirm only that module route shows the module error page.
8. Confirm the admin shell sidebar, module management page, and unrelated Core
   pages still work.
9. Open the provider account creation flow.
10. Confirm the mock account form emits a payload that Core wraps into the common
    account API with `platform=module`, `type=module`, and matching
    `provider_id`.

## Account Form Contributions

Provider account creation is driven by installed providers:

1. Core lists installed provider modules.
2. Admin selects a provider.
3. Frontend loads the provider account form contribution.
4. Module form returns `{ credential_type, credentials, module_config }`.
5. Core submits data to the common account API with `provider_id`.

No provider-specific fields should be added to the core account form after this protocol exists.
