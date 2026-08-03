-- Pre-defined install.* keys in fx's settings table (docs/spec/installation.md, "The
-- install settings"). settings.Set updates only, so the rows must exist before the
-- org-owner claim can fill them; empty value = not yet claimed.
INSERT INTO settings (key, value) VALUES
  ('install.org_id', ''),
  ('install.org_login', ''),
  ('install.installation_id', ''),
  ('install.installed_by_user_id', ''),
  ('install.installed_by_login', ''),
  ('install.installed_at', '')
ON CONFLICT (key) DO NOTHING;
