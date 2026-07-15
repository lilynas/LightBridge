-- Provider modules are an execution detail, not an account platform. The
-- initial OpenAI module migration accidentally replaced platform='openai'
-- with platform='module', which removed the account from native OpenAI
-- scheduling/refresh paths and made the UI treat it as an unknown platform.
--
-- Only rows carrying an explicit OpenAI provider-module marker are repaired.
-- Credentials and module metadata are preserved so the provider adapter keeps
-- working after the canonical platform is restored.
WITH repaired_accounts AS (
    UPDATE accounts
    SET platform = 'openai',
        sub_platform = '',
        extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object(
            'openai_platform_repair',
            jsonb_build_object(
                'repaired_at', NOW(),
                'previous_platform', platform,
                'previous_sub_platform', COALESCE(sub_platform, ''),
                'reason', 'provider module migration must preserve the canonical OpenAI platform'
            )
        ),
        updated_at = NOW()
    WHERE platform = 'module'
      AND LOWER(BTRIM(COALESCE(
          NULLIF(extra->>'provider_id', ''),
          NULLIF(extra->'module_migration'->>'provider_id', ''),
          ''
      ))) = 'openai'
    RETURNING id
)
UPDATE account_model_catalog AS catalog
SET platform = 'openai',
    updated_at = NOW()
FROM repaired_accounts
WHERE catalog.account_id = repaired_accounts.id
  AND catalog.platform = 'module';
