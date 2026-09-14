# Kitende Rotary Attendance Backend

Go/Gin backend for first-time registration, returning-visitor lookup, weekly attendance, custom Rotary clubs, buddy groups, and inviter/referral reporting.

## Key routes

| Method | Route | Purpose |
|---|---|---|
| `POST` | `/api/register` | First-time registration or returning guest check-in |
| `GET` | `/api/visitors/lookup?query=email-or-phone` | Returning visitor lookup |
| `POST` | `/api/visitors/lookup` | Returning visitor lookup using JSON |
| `GET` | `/api/clubs` | Reusable clubs collected from registrations |
| `GET` | `/api/attendance?date=YYYY-MM-DD` | Attendance rows plus summary/leaderboards |
| `GET` | `/api/attendance/summary?date=YYYY-MM-DD` | Summary and leaderboards without attendance rows |
| `GET` | `/api/ping` | Health check |

Omit `date` on either attendance endpoint to report across all stored attendance.

## Buddy-system registration payload

```json
{
  "fullName": "Peter Kato",
  "phone": "+256700000000",
  "email": "peter@example.com",
  "baseRotaryClub": "Rotary Club of Kitende",
  "buddyGroup": "group alpha",
  "invitedBy": "John Mugisha",
  "customClub": false,
  "rotaryClub": "Rotary Club of Kitende | GROUP ALPHA",
  "classification": "Rotarian",
  "purpose": "Club Fellowship",
  "attendanceDate": "2026-07-23"
}
```

The backend accepts both the structured fields and older clients that only send `rotaryClub`. It normalizes the values as follows:

- A missing prefix is added: `Kampala Central` becomes `Rotary Club of Kampala Central`.
- Buddy groups are uppercase: `group alpha` becomes `GROUP ALPHA`.
- The backwards-compatible display field is saved as `CLUB | GROUP`.
- A non-member cannot have a buddy group or be marked as a custom club.
- Custom clubs are persisted in `rotary_clubs` for reuse through `/api/clubs`.

## Attendance summary response

```json
{
  "totalAttendance": 18,
  "uniqueVisitors": 15,
  "customClubAttendance": 2,
  "buddyGroupAttendance": 12,
  "referredAttendance": 10,
  "topInviters": [
    {
      "name": "John Mugisha",
      "peopleConvinced": 7,
      "attendanceCredits": 9
    }
  ],
  "buddyGroups": [
    {
      "name": "GROUP ALPHA",
      "uniquePeople": 8,
      "attendanceCount": 10
    }
  ],
  "clubs": [
    {
      "name": "Rotary Club of Kitende",
      "uniquePeople": 11,
      "attendanceCount": 13
    }
  ]
}
```

`peopleConvinced` counts unique visitors. `attendanceCredits` records every attributed check-in, so a returning guest can contribute multiple attendance credits without being counted as several different people.

## Database migration

Run the migrations in this order:

```text
migrations/20260708_attendance_checkin.sql
migrations/20260723_buddy_custom_clubs.sql
migrations/20260914_club_operations.sql
```

The application also runs GORM AutoMigrate for attendance plus the club-operations tables at startup. Historical home-club attendance is used to backfill the member roster idempotently.

## Environment

Copy `.env.example` to `.env` and add the deployment secrets. The source ZIP deliberately does not include live credentials.

The backend currently reads the PostgreSQL connection from `dsn`.

## Admin protection

Set `ADMIN_API_KEY` in production to protect both attendance endpoints. The frontend/admin proxy must pass the same value through `X-Admin-API-Key`.

```bash
ADMIN_API_KEY=replace-with-a-long-random-secret
```

If `ADMIN_API_KEY` is omitted, the attendance endpoints remain open for backward compatibility during rollout.

## Run locally

```bash
go mod download
go run ./cmd/api
```

The API listens on port `8080`.

## Tests

```bash
go test ./...
```


## Club operations upgrade

The admin API now supports a persistent member roster, club-wide attendance analytics, buddy-group performance, meeting donations, measurable goals, Rotary projects, project income/expenses, invoice records, and queued/scheduled email campaigns.

Additional protected endpoints include:

- `GET /api/admin/dashboard`
- `GET|POST /api/admin/members` and `PATCH /api/admin/members/:id`
- `GET|POST /api/admin/donations`
- `GET|POST|PATCH /api/admin/goals`
- `GET|POST|PATCH /api/admin/projects` plus project `transactions` and `invoices`
- `GET|POST /api/admin/campaigns`
- `GET|POST /api/jobs/run` for the Go mail-job runner

### Mail jobs and Savara Mail

All automated and bulk mail is stored as durable PostgreSQL jobs and sent by Go. The long-running server checks the queue on an interval; the Vercel deployment also exposes `/api/jobs/run` for Vercel Cron. Each individual message deliberately authenticates against Savara Mail first and only then calls the send endpoint with the returned bearer token. Jobs are atomically claimed, stale claims are recovered, and failed sends retry up to three times.

Attendance creates an immediate registration-confirmation job and a next-day 09:00 Africa/Kampala thank-you job when the attendee has an email address. Visitors receive visitor-specific next-day copy and admins listed in `ADMIN_NOTIFICATION_EMAILS` receive a visitor alert.

Configure production from `.env.example`, especially `HOME_CLUB_NAMES`, `ADMIN_NOTIFICATION_EMAILS`, Savara Mail credentials, `ADMIN_API_KEY`, and `CRON_SECRET`.

## Frontend shows "club-operations API not found" or "backend initialization failed"

The expanded admin UI requires this backend version to be deployed. Verify `/api/ping` first, then verify an authenticated request to `/api/admin/dashboard`. Run `migrations/20260914_club_operations.sql` when your production database role does not have DDL permission for GORM AutoMigrate. The backend now returns JSON for initialization failures so the Next.js proxy can report a useful deployment error instead of an unreadable response.
