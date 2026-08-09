-- Attendance/check-in support for weekly Thursday fellowship and one-off events.
-- Safe to run before deploying the updated Go backend. GORM AutoMigrate will
-- also create these columns when the server starts, but this script is useful
-- for controlled production deployments.

ALTER TABLE register_records
  ADD COLUMN IF NOT EXISTS event_date varchar(100),
  ADD COLUMN IF NOT EXISTS attendance_date varchar(20),
  ADD COLUMN IF NOT EXISTS check_in_source varchar(50);

UPDATE register_records
SET attendance_date = COALESCE(attendance_date, to_char(created_at AT TIME ZONE 'Africa/Kampala', 'YYYY-MM-DD'))
WHERE attendance_date IS NULL OR attendance_date = '';

UPDATE register_records
SET event_date = COALESCE(event_date, attendance_date)
WHERE event_date IS NULL OR event_date = '';

CREATE INDEX IF NOT EXISTS idx_register_records_phone ON register_records(phone);
CREATE INDEX IF NOT EXISTS idx_register_records_email_lower ON register_records(LOWER(email));
CREATE INDEX IF NOT EXISTS idx_register_records_attendance_date ON register_records(attendance_date);
CREATE INDEX IF NOT EXISTS idx_register_records_attendance_event ON register_records(attendance_date, event, purpose);
