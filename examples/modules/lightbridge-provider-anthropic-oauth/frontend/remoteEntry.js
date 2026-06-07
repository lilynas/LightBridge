const style = `
.lb-anthropic-provider { font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; padding: 20px; color: #111827; }
.lb-anthropic-provider h2 { margin: 0 0 8px; font-size: 18px; font-weight: 650; }
.lb-anthropic-provider p { margin: 0 0 16px; color: #4b5563; font-size: 13px; line-height: 1.5; }
.lb-anthropic-provider label { display: block; margin: 12px 0 6px; color: #374151; font-size: 12px; font-weight: 600; }
.lb-anthropic-provider input, .lb-anthropic-provider textarea, .lb-anthropic-provider select { box-sizing: border-box; width: 100%; border: 1px solid #d1d5db; border-radius: 6px; padding: 9px 10px; font-size: 13px; background: #fff; }
.lb-anthropic-provider textarea { min-height: 86px; resize: vertical; }
.lb-anthropic-provider button { border: 1px solid #111827; border-radius: 6px; background: #111827; color: #fff; padding: 8px 10px; font-size: 13px; cursor: pointer; }
.lb-anthropic-provider a { color: #0f766e; font-size: 13px; word-break: break-all; }
.lb-anthropic-provider .row { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 12px; }
@media (max-width: 640px) { .lb-anthropic-provider .row { grid-template-columns: 1fr; } }
`;

const providerId = "anthropic-oauth";
const clientId = "9d1c250a-e61b-44d9-88ed-5944d1962f5e";
const redirectUri = "https://platform.claude.com/oauth/code/callback";
const fullScope = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload";
const inferenceScope = "user:inference";

function ensureStyle() {
  if (document.getElementById("lb-anthropic-provider-style")) return;
  const node = document.createElement("style");
  node.id = "lb-anthropic-provider-style";
  node.textContent = style;
  document.head.appendChild(node);
}

const AnthropicOAuthProviderSettings = {
  name: "AnthropicOAuthProviderSettings",
  mounted() {
    ensureStyle();
  },
  template: `
    <section class="lb-anthropic-provider">
      <h2>Anthropic OAuth Provider</h2>
      <p>Claude OAuth accounts created for this provider route through the module sidecar.</p>
    </section>
  `,
};

const AnthropicOAuthAccountForm = {
  name: "AnthropicOAuthAccountForm",
  props: {
    modelValue: { type: Object, default: () => ({}) },
  },
  emits: ["update:modelValue"],
  mounted() {
    ensureStyle();
    this.updateExtra("type", "oauth");
  },
  data() {
    return {
      authorizeUrl: "",
      selectedScope: fullScope,
      oauthMode: "full",
    };
  },
  methods: {
    updateSecret(key, value) {
      const current = this.modelValue || {};
      this.$emit("update:modelValue", {
        ...current,
        provider_id: providerId,
        module_id: providerId,
        credentials: {
          ...(current.credentials || {}),
          [key]: value,
        },
      });
    },
    updateExtra(key, value) {
      const current = this.modelValue || {};
      this.$emit("update:modelValue", {
        ...current,
        provider_id: providerId,
        module_id: providerId,
        extra: {
          ...(current.extra || {}),
          [key]: value,
        },
      });
    },
    setMode(mode) {
      this.oauthMode = mode;
      this.selectedScope = mode === "inference" ? inferenceScope : fullScope;
      this.updateExtra("oauth_scope", this.selectedScope);
      this.updateExtra("setup_token", mode === "inference");
    },
    async generateOAuthUrl() {
      const verifier = await generateCodeVerifier();
      const challenge = await generateCodeChallenge(verifier);
      const state = base64Url(randomBytes(32));
      const params = [
        ["code", "true"],
        ["client_id", clientId],
        ["response_type", "code"],
        ["redirect_uri", redirectUri],
        ["scope", this.selectedScope],
        ["code_challenge", challenge],
        ["code_challenge_method", "S256"],
        ["state", state],
      ];
      this.authorizeUrl = `https://claude.ai/oauth/authorize?${params.map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v).replace(/%20/g, "+")}`).join("&")}`;
      this.updateSecret("code_verifier", verifier);
      this.updateSecret("oauth_state", state);
      this.updateExtra("oauth_scope", this.selectedScope);
      this.updateExtra("redirect_uri", redirectUri);
      this.updateExtra("type", "oauth");
    },
  },
  template: `
    <section class="lb-anthropic-provider">
      <h2>Anthropic OAuth Account</h2>
      <p>Generate a Claude OAuth URL, paste the callback code, or paste migrated OAuth tokens.</p>
      <div class="row">
        <div>
          <label>OAuth Mode</label>
          <select :value="oauthMode" @change="setMode($event.target.value)">
            <option value="full">Full Claude Code OAuth</option>
            <option value="inference">Inference Setup Token</option>
          </select>
        </div>
        <div>
          <label>Base URL</label>
          <input autocomplete="off" spellcheck="false" placeholder="https://api.anthropic.com" @input="updateExtra('base_url', $event.target.value)" />
        </div>
      </div>
      <button type="button" @click="generateOAuthUrl">Generate OAuth URL</button>
      <p v-if="authorizeUrl"><a :href="authorizeUrl" target="_blank" rel="noreferrer">{{ authorizeUrl }}</a></p>
      <label>OAuth Callback Code</label>
      <textarea autocomplete="off" spellcheck="false" @input="updateSecret('authorization_code', $event.target.value)"></textarea>
      <label>Access Token</label>
      <textarea autocomplete="off" spellcheck="false" @input="updateSecret('access_token', $event.target.value)"></textarea>
      <label>Refresh Token</label>
      <textarea autocomplete="off" spellcheck="false" @input="updateSecret('refresh_token', $event.target.value)"></textarea>
      <label>Claude Session Key</label>
      <textarea autocomplete="off" spellcheck="false" @input="updateSecret('session_key', $event.target.value)"></textarea>
      <label>Proxy URL</label>
      <input autocomplete="off" spellcheck="false" placeholder="http://127.0.0.1:7890" @input="updateExtra('proxy_url', $event.target.value)" />
    </section>
  `,
};

function randomBytes(size) {
  const bytes = new Uint8Array(size);
  crypto.getRandomValues(bytes);
  return bytes;
}

async function generateCodeVerifier() {
  return base64Url(randomBytes(32));
}

async function generateCodeChallenge(verifier) {
  const data = new TextEncoder().encode(verifier);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return base64Url(new Uint8Array(digest));
}

function base64Url(bytes) {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

export { AnthropicOAuthProviderSettings, AnthropicOAuthAccountForm };
