-- Tenant-level settings: theme selection, custom accent colour, active module list.
-- Schema (JSONB): { "theme": "clinical", "customAccent": null, "activeModules": ["tpt-doctor"] }
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS settings JSONB NOT NULL DEFAULT '{}';
