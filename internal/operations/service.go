package operations

import (
	"context"
	"fmt"
	"html"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wksn753/kitende-rotary/internal/mail"
	"github.com/wksn753/kitende-rotary/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const nonMember = "I'm not a Rotarian / Non-member"

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) DB() *gorm.DB { return s.db }

// BackfillMembersFromAttendance seeds the roster from historical home-club
// attendance. It is safe to run repeatedly, but it should be invoked as an
// explicit maintenance task rather than during Vercel request-serving startup.
func (s *Service) BackfillMembersFromAttendance() error {
	if s == nil || s.db == nil {
		return nil
	}
	var records []models.RegisterRecord
	if err := s.db.Order("created_at ASC").Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		if !s.IsClubMember(record) {
			continue
		}
		if err := s.upsertMemberFromAttendance(record); err != nil {
			return err
		}
	}
	return nil
}

// AfterAttendance implements the optional registration automation hook used by
// VisitorHandler. It only persists jobs/roster updates; sending happens in the
// Go worker so registration stays fast and mail work is durable.
func (s *Service) AfterAttendance(record models.RegisterRecord) {
	if s == nil || s.db == nil {
		return
	}

	if s.IsClubMember(record) {
		if err := s.upsertMemberFromAttendance(record); err != nil {
			log.Printf("operations: member upsert failed for attendance %d: %v", record.ID, err)
		}
	}

	if record.Email != "" {
		if err := s.queueRegistrationConfirmation(record); err != nil {
			log.Printf("operations: queue registration confirmation failed for %d: %v", record.ID, err)
		}
		if err := s.queueNextDayThankYou(record); err != nil {
			log.Printf("operations: queue next-day thank-you failed for %d: %v", record.ID, err)
		}
	}

	if s.IsVisitor(record) {
		if err := s.queueAdminVisitorAlert(record); err != nil {
			log.Printf("operations: queue visitor admin alert failed for %d: %v", record.ID, err)
		}
	}
}

func (s *Service) IsClubMember(record models.RegisterRecord) bool {
	club := strings.TrimSpace(record.BaseRotaryClub)
	if club == "" {
		club = strings.TrimSpace(strings.Split(record.RotaryClub, "|")[0])
	}
	if club == "" || strings.EqualFold(club, nonMember) {
		return false
	}

	for _, home := range homeClubNames() {
		if strings.EqualFold(club, home) {
			return true
		}
	}
	return false
}

func (s *Service) IsVisitor(record models.RegisterRecord) bool {
	return !s.IsClubMember(record)
}

func homeClubNames() []string {
	raw := strings.TrimSpace(os.Getenv("HOME_CLUB_NAMES"))
	if raw == "" {
		raw = "Rotary Club of Kitende Breeze,Rotary Club of Nakawa"
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func ugandaLocation() *time.Location {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}

func parseAttendanceDate(value string, fallback time.Time) time.Time {
	loc := ugandaLocation()
	if parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), loc); err == nil {
		return parsed
	}
	return fallback.In(loc)
}

func (s *Service) upsertMemberFromAttendance(record models.RegisterRecord) error {
	now := record.SubmittedAt
	if now.IsZero() {
		now = time.Now().In(ugandaLocation())
	}

	var member models.ClubMember
	query := s.db.Where("1 = 0")
	if record.Email != "" && record.Phone != "" {
		query = s.db.Where("LOWER(email) = ? OR phone = ?", strings.ToLower(record.Email), record.Phone)
	} else if record.Email != "" {
		query = s.db.Where("LOWER(email) = ?", strings.ToLower(record.Email))
	} else if record.Phone != "" {
		query = s.db.Where("phone = ?", record.Phone)
	} else {
		query = s.db.Where("LOWER(full_name) = ?", strings.ToLower(record.FullName))
	}

	err := query.Order("id ASC").First(&member).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		member = models.ClubMember{
			FullName:       record.FullName,
			Phone:          record.Phone,
			Email:          strings.ToLower(record.Email),
			RotaryClub:     record.BaseRotaryClub,
			BuddyGroup:     record.BuddyGroup,
			Classification: record.Classification,
			LastSeenAt:     &now,
			Active:         true,
			Source:         "attendance",
		}
		return s.db.Create(&member).Error
	}

	member.FullName = record.FullName
	if record.Phone != "" {
		member.Phone = record.Phone
	}
	if record.Email != "" {
		member.Email = strings.ToLower(record.Email)
	}
	if record.BaseRotaryClub != "" {
		member.RotaryClub = record.BaseRotaryClub
	}
	if record.BuddyGroup != "" {
		member.BuddyGroup = record.BuddyGroup
	}
	if record.Classification != "" {
		member.Classification = record.Classification
	}
	member.LastSeenAt = &now
	member.Active = true
	if member.Source == "" {
		member.Source = "attendance"
	}
	return s.db.Save(&member).Error
}

func (s *Service) QueueEmailJob(job *models.EmailJob) error {
	if job == nil || strings.TrimSpace(job.RecipientEmail) == "" {
		return nil
	}
	job.RecipientEmail = strings.ToLower(strings.TrimSpace(job.RecipientEmail))
	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = time.Now().In(ugandaLocation())
	}
	if job.Status == "" {
		job.Status = "pending"
	}
	if job.DedupeKey == "" {
		job.DedupeKey = fmt.Sprintf("adhoc:%d:%s:%d", time.Now().UnixNano(), job.RecipientEmail, time.Now().Unix())
	}
	return s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dedupe_key"}}, DoNothing: true}).Create(job).Error
}

func (s *Service) queueRegistrationConfirmation(record models.RegisterRecord) error {
	htmlBody, err := mail.RenderTemplate("rotary-kitende-confirmation", map[string]any{
		"GUEST_NAME":        record.FullName,
		"EVENT_TIME":        envOr("EVENT_TIME", "4:00 PM"),
		"REGISTRATION_ID":   fmt.Sprintf("ROT-%d-%05d", time.Now().Year(), record.ID),
		"EVENT_DETAILS_URL": envOr("EVENT_DETAILS_URL", "https://rotary.siontravel.co.ug/"),
		"CONTACT_PHONE":     envOr("CLUB_CONTACT_PHONE", "+256759939977"),
	})
	if err != nil {
		return err
	}
	return s.QueueEmailJob(&models.EmailJob{
		JobType: "registration_confirmation", DedupeKey: fmt.Sprintf("attendance:%d:confirmation", record.ID),
		RecipientName: record.FullName, RecipientEmail: record.Email,
		Subject: "Attendance confirmed — Rotary Fellowship", Body: htmlBody,
		ScheduledAt: time.Now().In(ugandaLocation()),
	})
}

func (s *Service) queueNextDayThankYou(record models.RegisterRecord) error {
	attendanceDay := parseAttendanceDate(record.AttendanceDate, record.SubmittedAt)
	nextDay := time.Date(attendanceDay.Year(), attendanceDay.Month(), attendanceDay.Day()+1, 9, 0, 0, 0, ugandaLocation())

	subject := "Thank you for attending"
	body := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto"><h2>Thank you for joining us, %s.</h2><p>Thank you for attending %s on %s. Your presence made the fellowship stronger, and we hope to welcome you again soon.</p><p>With appreciation,<br><strong>%s</strong></p></div>`,
		html.EscapeString(record.FullName), html.EscapeString(record.Event), attendanceDay.Format("2 January 2006"), html.EscapeString(primaryHomeClubName()))
	jobType := "next_day_attendance_thank_you"
	if s.IsVisitor(record) {
		subject = "Thank you for visiting our Rotary fellowship"
		body = fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto"><h2>Thank you for visiting, %s.</h2><p>It was a pleasure welcoming you to %s on %s. We appreciate you taking the time to join us and hope this is the first of many visits.</p><p>Warm regards,<br><strong>%s</strong></p></div>`,
			html.EscapeString(record.FullName), html.EscapeString(record.Event), attendanceDay.Format("2 January 2006"), html.EscapeString(primaryHomeClubName()))
		jobType = "next_day_visitor_thank_you"
	}

	return s.QueueEmailJob(&models.EmailJob{
		JobType: jobType, DedupeKey: fmt.Sprintf("attendance:%d:next-day-thanks", record.ID),
		RecipientName: record.FullName, RecipientEmail: record.Email,
		Subject: subject, Body: body, ScheduledAt: nextDay,
	})
}

func (s *Service) queueAdminVisitorAlert(record models.RegisterRecord) error {
	admins := splitEmails(os.Getenv("ADMIN_NOTIFICATION_EMAILS"))
	for _, adminEmail := range admins {
		body := fmt.Sprintf(`<div style="font-family:Arial,sans-serif"><h2>New visitor attendance</h2><p><strong>%s</strong> checked in.</p><p>Club: %s<br>Email: %s<br>Phone: %s<br>Invited by: %s<br>Buddy group: %s</p></div>`,
			html.EscapeString(record.FullName), html.EscapeString(record.BaseRotaryClub), html.EscapeString(record.Email), html.EscapeString(record.Phone), html.EscapeString(record.InvitedBy), html.EscapeString(record.BuddyGroup))
		if err := s.QueueEmailJob(&models.EmailJob{
			JobType: "visitor_admin_alert", DedupeKey: fmt.Sprintf("attendance:%d:admin-alert:%s", record.ID, adminEmail),
			RecipientName: "Club Admin", RecipientEmail: adminEmail,
			Subject: "New visitor checked in: " + record.FullName, Body: body,
			ScheduledAt: time.Now().In(ugandaLocation()),
		}); err != nil {
			return err
		}
	}
	return nil
}

func primaryHomeClubName() string {
	names := homeClubNames()
	if len(names) == 0 {
		return "Rotary Club"
	}
	return names[0]
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func splitEmails(value string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	return out
}

type campaignRecipient struct{ Name, Email string }

func personalizeCampaignText(value string, recipient campaignRecipient) string {
	name := strings.TrimSpace(recipient.Name)
	firstName := name
	if fields := strings.Fields(name); len(fields) > 0 {
		firstName = fields[0]
	}
	replacer := strings.NewReplacer(
		"{{name}}", name,
		"{{first_name}}", firstName,
		"{{email}}", recipient.Email,
	)
	return replacer.Replace(value)
}

func (s *Service) CreateCampaign(campaign *models.EmailCampaign) error {
	if campaign == nil {
		return fmt.Errorf("campaign is required")
	}
	campaign.Name = strings.TrimSpace(campaign.Name)
	campaign.Audience = strings.TrimSpace(campaign.Audience)
	campaign.RecipientName = strings.TrimSpace(campaign.RecipientName)
	campaign.RecipientEmail = strings.ToLower(strings.TrimSpace(campaign.RecipientEmail))
	campaign.Subject = strings.TrimSpace(campaign.Subject)
	if campaign.Name == "" {
		campaign.Name = campaign.Subject
	}
	if campaign.Subject == "" || strings.TrimSpace(campaign.Body) == "" {
		return fmt.Errorf("subject and body are required")
	}
	if campaign.ScheduledAt.IsZero() {
		campaign.ScheduledAt = time.Now().In(ugandaLocation())
	}
	campaign.Status = "queued"

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(campaign).Error; err != nil {
			return err
		}
		var recipients []campaignRecipient
		var err error
		if campaign.Audience == "single" {
			if campaign.RecipientEmail == "" {
				return fmt.Errorf("recipientEmail is required for a single email")
			}
			recipients = []campaignRecipient{{Name: campaign.RecipientName, Email: campaign.RecipientEmail}}
		} else {
			recipients, err = recipientsForCampaign(tx, campaign.Audience, campaign.AttendanceDate, s)
			if err != nil {
				return err
			}
		}
		for index, recipient := range recipients {
			job := models.EmailJob{
				CampaignID: &campaign.ID, JobType: "campaign", DedupeKey: fmt.Sprintf("campaign:%d:%s:%d", campaign.ID, recipient.Email, index),
				RecipientName: recipient.Name, RecipientEmail: recipient.Email,
				Subject: personalizeCampaignText(campaign.Subject, recipient), Body: personalizeCampaignText(campaign.Body, recipient), ScheduledAt: campaign.ScheduledAt, Status: "pending",
			}
			if err := tx.Create(&job).Error; err != nil {
				return err
			}
		}
		if len(recipients) == 0 {
			now := time.Now().In(ugandaLocation())
			campaign.Status = "completed"
			campaign.CompletedAt = &now
			if err := tx.Save(campaign).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func recipientsForCampaign(db *gorm.DB, audience, attendanceDate string, service *Service) ([]campaignRecipient, error) {
	seen := map[string]struct{}{}
	out := make([]campaignRecipient, 0)
	appendRecipient := func(name, email string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			return
		}
		if _, ok := seen[email]; ok {
			return
		}
		seen[email] = struct{}{}
		out = append(out, campaignRecipient{Name: name, Email: email})
	}

	switch audience {
	case "members":
		var members []models.ClubMember
		if err := db.Where("active = ? AND email <> ''", true).Order("full_name ASC").Find(&members).Error; err != nil {
			return nil, err
		}
		for _, member := range members {
			appendRecipient(member.FullName, member.Email)
		}
	case "attendance_date":
		if strings.TrimSpace(attendanceDate) == "" {
			return nil, fmt.Errorf("attendanceDate is required for attendance_date audience")
		}
		var records []models.RegisterRecord
		if err := db.Where("attendance_date = ? AND email <> ''", attendanceDate).Order("created_at DESC").Find(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			appendRecipient(record.FullName, record.Email)
		}
	case "visitors":
		var records []models.RegisterRecord
		if err := db.Where("email <> ''").Order("created_at DESC").Find(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			if service.IsVisitor(record) {
				appendRecipient(record.FullName, record.Email)
			}
		}
	case "all_attendees", "":
		var records []models.RegisterRecord
		if err := db.Where("email <> ''").Order("created_at DESC").Find(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			appendRecipient(record.FullName, record.Email)
		}
	default:
		return nil, fmt.Errorf("unsupported audience %q", audience)
	}
	return out, nil
}

func (s *Service) RunDueJobs(ctx context.Context, limit int) (int, int, error) {
	if limit <= 0 {
		limit = 100
	}

	now := time.Now().In(ugandaLocation())
	// A process can die after claiming a job. Return old claims to the queue so
	// they are retried rather than staying permanently stuck in "processing".
	staleBefore := now.Add(-10 * time.Minute)
	if err := s.db.Model(&models.EmailJob{}).
		Where("status = ? AND updated_at < ?", "processing", staleBefore).
		Updates(map[string]any{"status": "pending", "last_error": "recovered stale worker claim"}).Error; err != nil {
		return 0, 0, err
	}

	var jobs []models.EmailJob
	if err := s.db.Where("status = ? AND scheduled_at <= ?", "pending", now).Order("scheduled_at ASC, id ASC").Limit(limit).Find(&jobs).Error; err != nil {
		return 0, 0, err
	}

	sent, failed := 0, 0
	campaignIDs := map[uint]struct{}{}
	for index := range jobs {
		job := &jobs[index]

		// Claim atomically. This prevents the long-running worker and a cron run
		// (or two serverless invocations) from sending the same email twice.
		claim := s.db.Model(&models.EmailJob{}).
			Where("id = ? AND status = ?", job.ID, "pending").
			Updates(map[string]any{"status": "processing", "attempts": gorm.Expr("attempts + 1")})
		if claim.Error != nil {
			return sent, failed, claim.Error
		}
		if claim.RowsAffected == 0 {
			continue
		}
		if err := s.db.First(job, job.ID).Error; err != nil {
			return sent, failed, err
		}

		if job.CampaignID != nil {
			campaignIDs[*job.CampaignID] = struct{}{}
		}

		err := mail.SendMailContext(ctx, job.RecipientEmail, job.Subject, job.Body)
		if err == nil {
			when := time.Now().In(ugandaLocation())
			job.Status = "sent"
			job.SentAt = &when
			job.LastError = ""
			sent++
		} else {
			job.LastError = err.Error()
			if job.Attempts >= 3 {
				job.Status = "failed"
				failed++
			} else {
				job.Status = "pending"
				job.ScheduledAt = time.Now().In(ugandaLocation()).Add(time.Duration(job.Attempts*15) * time.Minute)
			}
		}
		if saveErr := s.db.Save(job).Error; saveErr != nil {
			return sent, failed, saveErr
		}
	}

	for id := range campaignIDs {
		_ = s.refreshCampaign(id)
	}
	return sent, failed, nil
}

func (s *Service) refreshCampaign(id uint) error {
	var campaign models.EmailCampaign
	if err := s.db.First(&campaign, id).Error; err != nil {
		return err
	}
	var sentCount, failedCount, pendingCount int64
	base := s.db.Model(&models.EmailJob{}).Where("campaign_id = ?", id)
	if err := base.Where("status = ?", "sent").Count(&sentCount).Error; err != nil {
		return err
	}
	if err := s.db.Model(&models.EmailJob{}).Where("campaign_id = ? AND status = ?", id, "failed").Count(&failedCount).Error; err != nil {
		return err
	}
	if err := s.db.Model(&models.EmailJob{}).Where("campaign_id = ? AND status IN ?", id, []string{"pending", "processing"}).Count(&pendingCount).Error; err != nil {
		return err
	}
	campaign.SentCount, campaign.FailedCount = int(sentCount), int(failedCount)
	if pendingCount == 0 {
		when := time.Now().In(ugandaLocation())
		campaign.Status = "completed"
		campaign.CompletedAt = &when
	} else if sentCount > 0 || failedCount > 0 {
		campaign.Status = "sending"
	}
	return s.db.Save(&campaign).Error
}

func (s *Service) StartWorker(ctx context.Context) {
	interval := time.Minute
	if raw := strings.TrimSpace(os.Getenv("MAIL_JOB_INTERVAL_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 30 {
			interval = time.Duration(seconds) * time.Second
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				jobCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				_, _, err := s.RunDueJobs(jobCtx, 100)
				cancel()
				if err != nil {
					log.Printf("operations: mail worker: %v", err)
				}
			}
		}
	}()
}

func (s *Service) Dashboard() (models.OperationsDashboard, error) {
	out := models.OperationsDashboard{DonationCurrency: "UGX", TopAttendees: []models.PersonAttendanceLeaderboardEntry{}, BuddyGroups: []models.AttendanceLeaderboardEntry{}}
	var count int64
	if err := s.db.Model(&models.ClubMember{}).Count(&count).Error; err != nil {
		return out, err
	}
	out.TotalMembers = int(count)
	if err := s.db.Model(&models.ClubMember{}).Where("active = ?", true).Count(&count).Error; err != nil {
		return out, err
	}
	out.ActiveMembers = int(count)

	var records []models.RegisterRecord
	if err := s.db.Order("attendance_date DESC, created_at DESC").Find(&records).Error; err != nil {
		return out, err
	}
	out.TotalAttendance = len(records)
	visitorCount := 0
	personMap := map[string]*models.PersonAttendanceLeaderboardEntry{}
	unique := map[string]struct{}{}
	groupMap := map[string]*groupAccumulator{}
	for _, record := range records {
		key := identity(record)
		unique[key] = struct{}{}
		if s.IsVisitor(record) {
			visitorCount++
			continue
		}

		// Club attendance and buddy-group leaderboards should measure the home
		// club rather than allowing a frequent visitor to top a member ranking.
		entry := personMap[key]
		if entry == nil {
			entry = &models.PersonAttendanceLeaderboardEntry{Name: record.FullName, Email: record.Email, Phone: record.Phone, RotaryClub: record.BaseRotaryClub, BuddyGroup: record.BuddyGroup}
			personMap[key] = entry
		}
		entry.AttendanceCount++
		if record.AttendanceDate > entry.LastAttendance {
			entry.LastAttendance = record.AttendanceDate
		}
		if group := strings.TrimSpace(record.BuddyGroup); group != "" {
			keyGroup := strings.ToLower(group)
			acc := groupMap[keyGroup]
			if acc == nil {
				acc = &groupAccumulator{name: group, people: map[string]struct{}{}}
				groupMap[keyGroup] = acc
			}
			acc.attendance++
			acc.people[key] = struct{}{}
		}
	}
	out.UniqueAttendees = len(unique)
	out.VisitorAttendance = visitorCount
	for _, entry := range personMap {
		out.TopAttendees = append(out.TopAttendees, *entry)
	}
	sort.Slice(out.TopAttendees, func(i, j int) bool {
		if out.TopAttendees[i].AttendanceCount != out.TopAttendees[j].AttendanceCount {
			return out.TopAttendees[i].AttendanceCount > out.TopAttendees[j].AttendanceCount
		}
		return strings.ToLower(out.TopAttendees[i].Name) < strings.ToLower(out.TopAttendees[j].Name)
	})
	if len(out.TopAttendees) > 20 {
		out.TopAttendees = out.TopAttendees[:20]
	}
	for _, acc := range groupMap {
		out.BuddyGroups = append(out.BuddyGroups, models.AttendanceLeaderboardEntry{Name: acc.name, UniquePeople: len(acc.people), AttendanceCount: acc.attendance})
	}
	sort.Slice(out.BuddyGroups, func(i, j int) bool { return out.BuddyGroups[i].AttendanceCount > out.BuddyGroups[j].AttendanceCount })

	var totalDonations struct{ Total float64 }
	if err := s.db.Model(&models.Donation{}).Select("COALESCE(SUM(amount), 0) AS total").Scan(&totalDonations).Error; err != nil {
		return out, err
	}
	out.TotalDonations = totalDonations.Total
	if err := s.db.Model(&models.ClubGoal{}).Where("status IN ?", []string{"active", "in_progress"}).Count(&count).Error; err != nil {
		return out, err
	}
	out.ActiveGoals = int(count)
	if err := s.db.Model(&models.RotaryProject{}).Where("status NOT IN ?", []string{"completed", "cancelled"}).Count(&count).Error; err != nil {
		return out, err
	}
	out.ActiveProjects = int(count)
	if err := s.db.Model(&models.EmailJob{}).Where("status = ?", "pending").Count(&count).Error; err != nil {
		return out, err
	}
	out.PendingMailJobs = int(count)
	return out, nil
}

type groupAccumulator struct {
	name       string
	attendance int
	people     map[string]struct{}
}

func identity(record models.RegisterRecord) string {
	if strings.TrimSpace(record.Email) != "" {
		return "email:" + strings.ToLower(strings.TrimSpace(record.Email))
	}
	if strings.TrimSpace(record.Phone) != "" {
		return "phone:" + strings.TrimSpace(record.Phone)
	}
	return "name:" + strings.ToLower(strings.TrimSpace(record.FullName))
}
