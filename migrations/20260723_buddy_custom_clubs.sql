-- Rotary Club of Kitende Breeze
-- Custom Rotary clubs, buddy groups, inviter attribution, and reusable club directory.
-- Safe to run more than once.

BEGIN;

ALTER TABLE register_records
  ADD COLUMN IF NOT EXISTS base_rotary_club varchar(255),
  ADD COLUMN IF NOT EXISTS buddy_group varchar(120),
  ADD COLUMN IF NOT EXISTS invited_by varchar(255),
  ADD COLUMN IF NOT EXISTS custom_club boolean NOT NULL DEFAULT false;

-- Backfill structured values from older display values such as:
--   Rotary Club of Kitende | GROUP ALPHA
UPDATE register_records
SET base_rotary_club = NULLIF(BTRIM(SPLIT_PART(rotary_club, '|', 1)), '')
WHERE (base_rotary_club IS NULL OR BTRIM(base_rotary_club) = '')
  AND rotary_club IS NOT NULL
  AND BTRIM(rotary_club) <> '';

UPDATE register_records
SET buddy_group = NULLIF(
  UPPER(
    BTRIM(
      CASE
        WHEN POSITION('|' IN rotary_club) > 0
          THEN SUBSTRING(rotary_club FROM POSITION('|' IN rotary_club) + 1)
        ELSE ''
      END
    )
  ),
  ''
)
WHERE (buddy_group IS NULL OR BTRIM(buddy_group) = '')
  AND rotary_club IS NOT NULL
  AND POSITION('|' IN rotary_club) > 0;

UPDATE register_records
SET buddy_group = UPPER(BTRIM(buddy_group))
WHERE buddy_group IS NOT NULL AND BTRIM(buddy_group) <> '';

-- Keep the backwards-compatible display field synchronized.
UPDATE register_records
SET rotary_club = CASE
  WHEN buddy_group IS NOT NULL AND BTRIM(buddy_group) <> ''
    THEN BTRIM(base_rotary_club) || ' | ' || UPPER(BTRIM(buddy_group))
  ELSE BTRIM(base_rotary_club)
END
WHERE base_rotary_club IS NOT NULL AND BTRIM(base_rotary_club) <> '';

CREATE INDEX IF NOT EXISTS idx_register_records_base_rotary_club
  ON register_records(base_rotary_club);

CREATE INDEX IF NOT EXISTS idx_register_records_buddy_group
  ON register_records(buddy_group);

CREATE INDEX IF NOT EXISTS idx_register_records_invited_by
  ON register_records(invited_by);

CREATE INDEX IF NOT EXISTS idx_register_records_custom_club
  ON register_records(custom_club);

CREATE TABLE IF NOT EXISTS rotary_clubs (
  id bigserial PRIMARY KEY,
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz,
  name varchar(255) NOT NULL,
  normalized_name varchar(255) NOT NULL,
  is_custom boolean NOT NULL DEFAULT true,
  active boolean NOT NULL DEFAULT true
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rotary_clubs_normalized_name
  ON rotary_clubs(normalized_name);

CREATE INDEX IF NOT EXISTS idx_rotary_clubs_deleted_at
  ON rotary_clubs(deleted_at);

CREATE INDEX IF NOT EXISTS idx_rotary_clubs_active
  ON rotary_clubs(active);

-- Seed the reusable directory from historical attendance records.
INSERT INTO rotary_clubs (
  created_at,
  updated_at,
  name,
  normalized_name,
  is_custom,
  active
)
SELECT
  NOW(),
  NOW(),
  MIN(BTRIM(base_rotary_club)),
  LOWER(BTRIM(base_rotary_club)),
  BOOL_OR(custom_club),
  true
FROM register_records
WHERE base_rotary_club IS NOT NULL
  AND BTRIM(base_rotary_club) <> ''
  AND LOWER(BTRIM(base_rotary_club)) <> LOWER('I''m not a Rotarian / Non-member')
GROUP BY LOWER(BTRIM(base_rotary_club))
ON CONFLICT (normalized_name) DO UPDATE
SET
  name = EXCLUDED.name,
  active = true,
  is_custom = rotary_clubs.is_custom OR EXCLUDED.is_custom,
  updated_at = NOW();

COMMIT;
