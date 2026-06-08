# UI Theme Agent Configuration

This document is for AI agents and automation scripts that need to configure LightBridge UI themes remotely.

## Authentication

Use an Admin API Key from the admin settings page.

Required headers:

```bash
export LIGHTBRIDGE_BASE_URL="https://lightbridge.example.com"
export LIGHTBRIDGE_ADMIN_API_KEY="admin-<64hex>"
```

Every Admin API request must send:

```text
x-api-key: admin-<64hex>
Content-Type: application/json
```

## Theme Package

ZIP file or GitHub repository root:

```text
lightbridge-ui.json
style.css
preview.png
pages/welcome.md
images/logo.png
fonts/inter.woff2
```

`lightbridge-ui.json`:

```json
{
  "id": "modern-light",
  "name": "Modern Light",
  "version": "1.0.0",
  "entry_css": "style.css",
  "preview": "preview.png",
  "config": [
    { "key": "primary_color", "label": "Primary Color", "type": "color", "default": "#2563eb" },
    { "key": "radius", "label": "Radius", "type": "text", "default": "8px" }
  ],
  "menu_items": [
    {
      "id": "welcome",
      "label": "Welcome",
      "visibility": "user",
      "type": "markdown",
      "source": "pages/welcome.md",
      "sort_order": 100
    },
    {
      "id": "status",
      "label": "Status",
      "visibility": "admin",
      "type": "iframe",
      "url": "https://status.example.com",
      "sort_order": 110
    }
  ]
}
```

Supported config field types: `color`, `text`, `select`, `number`, `boolean`.

## CLI

Build:

```bash
cd backend
go build -o ../lightbridge-ui-theme ./cmd/ui-theme
```

List themes:

```bash
./lightbridge-ui-theme list --json
```

Install from GitHub and activate:

```bash
./lightbridge-ui-theme apply \
  --github https://github.com/org/lightbridge-theme \
  --activate \
  --json
```

Install from ZIP with config:

```bash
./lightbridge-ui-theme apply \
  --zip ./theme.zip \
  --config ./theme-config.json \
  --activate \
  --json
```

Replace an existing theme:

```bash
./lightbridge-ui-theme apply \
  --zip ./theme.zip \
  --replace \
  --activate \
  --json
```

Configure:

```bash
./lightbridge-ui-theme configure \
  --theme modern-light \
  --config ./theme-config.json \
  --json
```

Activate:

```bash
./lightbridge-ui-theme activate --theme modern-light --json
```

Deactivate:

```bash
./lightbridge-ui-theme deactivate --theme modern-light --json
```

Delete:

```bash
./lightbridge-ui-theme delete --theme modern-light --json
```

Validate local package size:

```bash
./lightbridge-ui-theme validate-package --zip ./theme.zip --json
```

## Curl Playbook

Install from GitHub:

```bash
curl -sS -X POST "${LIGHTBRIDGE_BASE_URL}/api/v1/admin/ui-themes/import-github" \
  -H "x-api-key: ${LIGHTBRIDGE_ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://github.com/org/lightbridge-theme","replace":false}'
```

Upload ZIP:

```bash
curl -sS -X POST "${LIGHTBRIDGE_BASE_URL}/api/v1/admin/ui-themes/upload" \
  -H "x-api-key: ${LIGHTBRIDGE_ADMIN_API_KEY}" \
  -F "file=@./theme.zip"
```

Activate:

```bash
curl -sS -X PUT "${LIGHTBRIDGE_BASE_URL}/api/v1/admin/ui-themes/modern-light/activate" \
  -H "x-api-key: ${LIGHTBRIDGE_ADMIN_API_KEY}"
```

Configure:

```bash
curl -sS -X PUT "${LIGHTBRIDGE_BASE_URL}/api/v1/admin/ui-themes/modern-light/config" \
  -H "x-api-key: ${LIGHTBRIDGE_ADMIN_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"config":{"primary_color":"#16a34a","radius":"6px"}}'
```

Rollback:

```bash
curl -sS -X PUT "${LIGHTBRIDGE_BASE_URL}/api/v1/admin/ui-themes/modern-light/deactivate" \
  -H "x-api-key: ${LIGHTBRIDGE_ADMIN_API_KEY}"
```

Delete:

```bash
curl -sS -X DELETE "${LIGHTBRIDGE_BASE_URL}/api/v1/admin/ui-themes/modern-light" \
  -H "x-api-key: ${LIGHTBRIDGE_ADMIN_API_KEY}"
```

## JSON Output

CLI success:

```json
{
  "ok": true,
  "action": "apply",
  "data": {
    "code": 0,
    "message": "success",
    "data": {
      "id": "modern-light"
    }
  }
}
```

CLI failure:

```json
{
  "ok": false,
  "action": "apply",
  "error": {
    "message": "HTTP 401: Authorization required"
  }
}
```

Exit codes:

- `0`: success
- `1`: remote API or runtime failure
- `2`: invalid command or missing arguments

## Security Rules

- No JavaScript, Vue components, inline script, or arbitrary HTML execution.
- CSS cannot contain `javascript:`, `expression()`, `behavior`, `-moz-binding`, `@import`, external URLs, or data URLs.
- ZIP max size: 10MB.
- Extracted package max size: 20MB.
- Single CSS max size: 512KB.
- Allowed files: `.json`, `.css`, `.md`, `.png`, `.jpg`, `.jpeg`, `.svg`, `.webp`, `.woff`, `.woff2`.
- Paths cannot be absolute, hidden, symlinked, or traverse outside the package.
