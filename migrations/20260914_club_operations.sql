-- Club operations expansion: members, donations, goals, Rotary projects,
-- finance/invoices, and durable email campaigns/jobs.
-- Safe to run more than once; GORM AutoMigrate also creates these tables.

CREATE TABLE IF NOT EXISTS club_members (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  full_name varchar(255) NOT NULL,
  phone varchar(32),
  email varchar(255),
  rotary_club varchar(255),
  buddy_group varchar(120),
  classification varchar(255),
  joined_at timestamptz,
  last_seen_at timestamptz,
  active boolean NOT NULL DEFAULT true,
  source varchar(50)
);
CREATE INDEX IF NOT EXISTS idx_club_members_name ON club_members(full_name);
CREATE INDEX IF NOT EXISTS idx_club_members_email_lower ON club_members(LOWER(email));
CREATE INDEX IF NOT EXISTS idx_club_members_phone ON club_members(phone);
CREATE INDEX IF NOT EXISTS idx_club_members_buddy_group ON club_members(buddy_group);
CREATE INDEX IF NOT EXISTS idx_club_members_active ON club_members(active);
CREATE INDEX IF NOT EXISTS idx_club_members_deleted_at ON club_members(deleted_at);

CREATE TABLE IF NOT EXISTS donations (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  attendance_date varchar(20) NOT NULL,
  donor_name varchar(255) NOT NULL,
  donor_email varchar(255),
  donor_phone varchar(32),
  member_id bigint,
  amount numeric(18,2) NOT NULL,
  currency varchar(8) NOT NULL DEFAULT 'UGX',
  payment_method varchar(50),
  reference varchar(120),
  notes text,
  recorded_by varchar(120)
);
CREATE INDEX IF NOT EXISTS idx_donations_attendance_date ON donations(attendance_date);
CREATE INDEX IF NOT EXISTS idx_donations_member_id ON donations(member_id);
CREATE INDEX IF NOT EXISTS idx_donations_deleted_at ON donations(deleted_at);

CREATE TABLE IF NOT EXISTS club_goals (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  title varchar(255) NOT NULL,
  description text,
  metric varchar(120),
  target_value numeric(18,2),
  current_value numeric(18,2),
  unit varchar(50),
  start_date varchar(20),
  due_date varchar(20),
  status varchar(30) DEFAULT 'active'
);
CREATE INDEX IF NOT EXISTS idx_club_goals_status ON club_goals(status);
CREATE INDEX IF NOT EXISTS idx_club_goals_due_date ON club_goals(due_date);
CREATE INDEX IF NOT EXISTS idx_club_goals_deleted_at ON club_goals(deleted_at);

CREATE TABLE IF NOT EXISTS rotary_projects (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  name varchar(255) NOT NULL,
  description text,
  status varchar(30) DEFAULT 'planned',
  start_date varchar(20),
  end_date varchar(20),
  budget numeric(18,2),
  currency varchar(8) DEFAULT 'UGX',
  goal_id bigint
);
CREATE INDEX IF NOT EXISTS idx_rotary_projects_status ON rotary_projects(status);
CREATE INDEX IF NOT EXISTS idx_rotary_projects_goal_id ON rotary_projects(goal_id);
CREATE INDEX IF NOT EXISTS idx_rotary_projects_deleted_at ON rotary_projects(deleted_at);

CREATE TABLE IF NOT EXISTS project_transactions (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  project_id bigint NOT NULL,
  type varchar(20) NOT NULL,
  amount numeric(18,2) NOT NULL,
  currency varchar(8) DEFAULT 'UGX',
  category varchar(120),
  description text,
  transaction_date varchar(20),
  reference varchar(120),
  recorded_by varchar(120)
);
CREATE INDEX IF NOT EXISTS idx_project_transactions_project_id ON project_transactions(project_id);
CREATE INDEX IF NOT EXISTS idx_project_transactions_type ON project_transactions(type);
CREATE INDEX IF NOT EXISTS idx_project_transactions_date ON project_transactions(transaction_date);
CREATE INDEX IF NOT EXISTS idx_project_transactions_deleted_at ON project_transactions(deleted_at);

CREATE TABLE IF NOT EXISTS project_invoices (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  project_id bigint NOT NULL,
  invoice_number varchar(120) NOT NULL,
  vendor varchar(255),
  customer varchar(255),
  description text,
  amount numeric(18,2) NOT NULL,
  currency varchar(8) DEFAULT 'UGX',
  status varchar(30) DEFAULT 'unpaid',
  issue_date varchar(20),
  due_date varchar(20),
  paid_at timestamptz,
  file_url text
);
CREATE INDEX IF NOT EXISTS idx_project_invoices_project_id ON project_invoices(project_id);
CREATE INDEX IF NOT EXISTS idx_project_invoices_invoice_number ON project_invoices(invoice_number);
CREATE INDEX IF NOT EXISTS idx_project_invoices_status ON project_invoices(status);
CREATE INDEX IF NOT EXISTS idx_project_invoices_deleted_at ON project_invoices(deleted_at);

CREATE TABLE IF NOT EXISTS email_campaigns (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  name varchar(255) NOT NULL,
  audience varchar(50) NOT NULL,
  subject varchar(255) NOT NULL,
  body text NOT NULL,
  attendance_date varchar(20),
  scheduled_at timestamptz,
  status varchar(30) DEFAULT 'queued',
  created_by varchar(120),
  sent_count integer DEFAULT 0,
  failed_count integer DEFAULT 0,
  completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_email_campaigns_status ON email_campaigns(status);
CREATE INDEX IF NOT EXISTS idx_email_campaigns_scheduled_at ON email_campaigns(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_email_campaigns_deleted_at ON email_campaigns(deleted_at);

CREATE TABLE IF NOT EXISTS email_jobs (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  campaign_id bigint,
  job_type varchar(60) NOT NULL,
  dedupe_key varchar(255) NOT NULL,
  recipient_name varchar(255),
  recipient_email varchar(255) NOT NULL,
  subject varchar(255) NOT NULL,
  body text NOT NULL,
  scheduled_at timestamptz NOT NULL,
  status varchar(30) NOT NULL DEFAULT 'pending',
  attempts integer NOT NULL DEFAULT 0,
  last_error text,
  sent_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_jobs_dedupe_key ON email_jobs(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_email_jobs_due ON email_jobs(status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_email_jobs_campaign_id ON email_jobs(campaign_id);
CREATE INDEX IF NOT EXISTS idx_email_jobs_deleted_at ON email_jobs(deleted_at);
