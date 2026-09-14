-- Communications UX upgrade: single-recipient email and reusable visual templates.
-- Safe to run repeatedly.

ALTER TABLE email_campaigns ADD COLUMN IF NOT EXISTS recipient_name varchar(255);
ALTER TABLE email_campaigns ADD COLUMN IF NOT EXISTS recipient_email varchar(255);
ALTER TABLE email_campaigns ADD COLUMN IF NOT EXISTS template_id bigint;
CREATE INDEX IF NOT EXISTS idx_email_campaigns_recipient_email ON email_campaigns(recipient_email);
CREATE INDEX IF NOT EXISTS idx_email_campaigns_template_id ON email_campaigns(template_id);

CREATE TABLE IF NOT EXISTS email_templates (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  name varchar(255) NOT NULL,
  description text,
  subject varchar(255),
  preheader varchar(255),
  accent_color varchar(20) DEFAULT '#17458f',
  logo_url text,
  partner_logo_url text,
  hero_image_url text,
  heading varchar(255),
  body_text text,
  button_label varchar(120),
  button_url text,
  footer_text text,
  created_by varchar(120)
);
CREATE INDEX IF NOT EXISTS idx_email_templates_name ON email_templates(name);
CREATE INDEX IF NOT EXISTS idx_email_templates_deleted_at ON email_templates(deleted_at);

ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS partner_logo_url text;
