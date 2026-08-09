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
```

The application also runs GORM AutoMigrate for `register_records` and `rotary_clubs` at startup.

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
