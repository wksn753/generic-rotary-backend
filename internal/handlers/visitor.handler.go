package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/wksn753/kitende-rotary/internal/mail"
	"github.com/wksn753/kitende-rotary/internal/models"
	"github.com/wksn753/kitende-rotary/internal/repository"
	"gorm.io/gorm"
)

// Event constants used to populate the confirmation email.
// Move these into config/env if they might change before the ceremony.
const (
	eventTime       = "4:00 PM"
	eventDetailsURL = "https://rotary.siontravel.co.ug/"
	contactPhone    = "+256759939977"
	nonMember       = "I'm not a Rotarian / Non-member"
)

var nonDigitRegex = regexp.MustCompile(`\D+`)

type VisitorHandler struct {
	infrastructure repository.VisitorRepository
}

type lookupRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
	Query string `json:"query"`
}

func NewVisitorHandler(infrastructure repository.VisitorRepository) *VisitorHandler {
	return &VisitorHandler{infrastructure: infrastructure}
}

func (v *VisitorHandler) RegisterVisitor(c *gin.Context) {
	var req models.RegisterRecord

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "VALIDATION",
			"message": "Invalid request payload",
		})
		return
	}

	if message := validateRawRecord(&req); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "VALIDATION",
			"message": message,
		})
		return
	}

	normaliseRecord(&req)

	// Returning visitor shortcut: if the guest only entered phone/email,
	// reuse their latest saved profile instead of forcing them to fill the
	// same club/classification/name details every Thursday.
	if req.FullName == "" && (req.Email != "" || req.Phone != "") {
		previous, err := v.infrastructure.FindReturningVisitor(req.Email, req.Phone)
		if err != nil && err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "LOOKUP_FAILED", "message": "Failed to check returning visitor"})
			return
		}

		if previous != nil {
			mergeReturningVisitorDetails(&req, previous)
			normaliseRecord(&req)
		}
	}

	if message := validateRecord(&req); message != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "VALIDATION",
			"message": message,
		})
		return
	}

	alreadyCheckedIn, err := v.infrastructure.FindSameDayAttendance(
		req.Email,
		req.Phone,
		req.FullName,
		req.AttendanceDate,
		req.Event,
		req.Purpose,
	)
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "DUPLICATE_CHECK_FAILED",
			"message": "Failed to check today's attendance",
		})
		return
	}

	if alreadyCheckedIn != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":           true,
			"alreadyRegistered": true,
			"message":           fmt.Sprintf("%s is already checked in for %s", alreadyCheckedIn.FullName, alreadyCheckedIn.AttendanceDate),
			"record":            alreadyCheckedIn,
		})
		return
	}

	if err := v.infrastructure.SaveVisitorRecord(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    "SAVE_FAILED",
			"message": "Failed to register visitor",
		})
		return
	}

	// Keep the club directory current without allowing a directory failure to
	// undo an attendance record that has already been committed.
	if req.BaseRotaryClub != "" && !strings.EqualFold(req.BaseRotaryClub, nonMember) {
		if err := v.infrastructure.UpsertRotaryClub(req.BaseRotaryClub, req.CustomClub); err != nil {
			log.Printf("clubs: failed to persist %q after registration %d: %v", req.BaseRotaryClub, req.ID, err)
		}
	}

	// Registration is already committed at this point — nothing past here
	// should be able to turn this into a failure response.
	if req.Email != "" {
		go sendConfirmationAsync(req.ID, req.Email, req.FullName)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("%s checked in successfully", req.FullName),
		"record":  req,
	})
}

func (v *VisitorHandler) LookupVisitor(c *gin.Context) {
	var req lookupRequest

	if c.Request.Method == http.MethodGet {
		req.Query = c.Query("query")
		req.Email = c.Query("email")
		req.Phone = c.Query("phone")
	} else if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "VALIDATION", "message": "Invalid request payload"})
		return
	}

	email, phone := splitLookupContact(req.Query, req.Email, req.Phone)
	if email == "" && phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "VALIDATION", "message": "Enter an email address or phone number"})
		return
	}

	record, err := v.infrastructure.FindReturningVisitor(email, phone)
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "NOT_FOUND", "message": "No previous registration found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "LOOKUP_FAILED", "message": "Failed to find returning visitor"})
		return
	}

	normaliseRecord(record)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"visitor": models.ReturningVisitor{
			ID:             record.ID,
			FullName:       record.FullName,
			Phone:          record.Phone,
			Email:          record.Email,
			RotaryClub:     record.RotaryClub,
			BaseRotaryClub: record.BaseRotaryClub,
			BuddyGroup:     record.BuddyGroup,
			InvitedBy:      record.InvitedBy,
			CustomClub:     record.CustomClub,
			Classification: record.Classification,
		},
	})
}

func (v *VisitorHandler) GetRotaryClubs(c *gin.Context) {
	clubs, err := v.infrastructure.GetRotaryClubs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "FETCH_FAILED", "message": "Failed to fetch Rotary clubs"})
		return
	}

	options := make([]models.RotaryClubOption, 0, len(clubs))
	for _, club := range clubs {
		options = append(options, models.RotaryClubOption{
			Name:     club.Name,
			IsCustom: club.IsCustom,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(options),
		"clubs":   options,
	})
}

func (v *VisitorHandler) GetAttendance(c *gin.Context) {
	attendanceDate := strings.TrimSpace(c.Query("date"))

	records, err := v.infrastructure.GetVisitorRecordsByDate(attendanceDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "FETCH_FAILED", "message": "Failed to fetch attendance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"date":    attendanceDate,
		"count":   len(records),
		"summary": buildAttendanceSummary(records),
		"records": records,
	})
}

func (v *VisitorHandler) GetAttendanceSummary(c *gin.Context) {
	attendanceDate := strings.TrimSpace(c.Query("date"))

	records, err := v.infrastructure.GetVisitorRecordsByDate(attendanceDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "FETCH_FAILED", "message": "Failed to fetch attendance summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"date":    attendanceDate,
		"summary": buildAttendanceSummary(records),
	})
}

func normaliseRecord(record *models.RegisterRecord) {
	record.FullName = cleanText(record.FullName)
	record.Email = normaliseEmail(record.Email)
	record.Phone = normalisePhone(record.Phone)
	record.Classification = cleanText(record.Classification)
	record.Purpose = cleanText(record.Purpose)
	record.OtherPurpose = cleanText(record.OtherPurpose)
	record.Event = cleanText(record.Event)
	record.EventDate = cleanText(record.EventDate)
	record.AttendanceDate = cleanText(record.AttendanceDate)
	record.Venue = cleanText(record.Venue)
	record.CheckInSource = cleanText(record.CheckInSource)
	record.InvitedBy = cleanText(record.InvitedBy)

	clubFromDisplay, groupFromDisplay := splitClubAndBuddyGroup(record.RotaryClub)
	baseClubFromField, groupFromBaseField := splitClubAndBuddyGroup(record.BaseRotaryClub)

	baseClub := baseClubFromField
	if baseClub == "" {
		baseClub = clubFromDisplay
	}

	buddyGroup := record.BuddyGroup
	if cleanText(buddyGroup) == "" {
		if groupFromBaseField != "" {
			buddyGroup = groupFromBaseField
		} else {
			buddyGroup = groupFromDisplay
		}
	}

	record.BaseRotaryClub = normaliseClubName(baseClub)
	record.BuddyGroup = strings.ToUpper(cleanText(buddyGroup))

	if strings.EqualFold(record.BaseRotaryClub, nonMember) {
		record.BaseRotaryClub = nonMember
		record.BuddyGroup = ""
		record.CustomClub = false
	}

	record.RotaryClub = combineClubAndBuddyGroup(record.BaseRotaryClub, record.BuddyGroup)

	if record.Purpose == "" {
		record.Purpose = "Club Fellowship"
	}
	if record.Event == "" {
		record.Event = eventNameForPurpose(record.Purpose)
	}
	if record.Venue == "" {
		record.Venue = "Rotary Club of Kitende Breeze"
	}
	if record.AttendanceDate == "" {
		record.AttendanceDate = time.Now().In(ugandaLocation()).Format("2006-01-02")
	}
	if record.EventDate == "" {
		record.EventDate = record.AttendanceDate
	}
	if record.CheckInSource == "" {
		record.CheckInSource = "web"
	}
	if record.SubmittedAt.IsZero() {
		record.SubmittedAt = time.Now().In(ugandaLocation())
	}
}

func validateRawRecord(record *models.RegisterRecord) string {
	values := map[string]string{
		"full name":        record.FullName,
		"Rotary club":      record.RotaryClub,
		"base Rotary club": record.BaseRotaryClub,
		"buddy group":      record.BuddyGroup,
		"invited by":       record.InvitedBy,
	}

	for label, value := range values {
		if containsControlCharacter(value) {
			return fmt.Sprintf("%s contains unsupported characters", label)
		}
	}

	return ""
}

func validateRecord(record *models.RegisterRecord) string {
	if record.FullName == "" {
		return "Full name is required for first-time visitors. Returning visitors can enter email or phone first to retrieve their details."
	}
	if record.Email == "" && record.Phone == "" {
		return "Please provide at least an email address or phone number so future fellowship check-ins are quick."
	}
	if record.BaseRotaryClub == "" {
		return "Please select your Rotary club, add a missing club, or choose non-member."
	}

	limits := []struct {
		label string
		value string
		max   int
	}{
		{"Full name", record.FullName, 255},
		{"Rotary club", record.BaseRotaryClub, 255},
		{"Buddy group", record.BuddyGroup, 120},
		{"Invited by", record.InvitedBy, 255},
		{"Classification", record.Classification, 255},
		{"Purpose", record.Purpose, 255},
	}

	for _, item := range limits {
		if len([]rune(item.value)) > item.max {
			return fmt.Sprintf("%s must be %d characters or fewer", item.label, item.max)
		}
	}

	if _, err := time.Parse("2006-01-02", record.AttendanceDate); err != nil {
		return "Attendance date must use YYYY-MM-DD format"
	}

	return ""
}

func mergeReturningVisitorDetails(target *models.RegisterRecord, previous *models.RegisterRecord) {
	if target.FullName == "" {
		target.FullName = previous.FullName
	}
	if target.Email == "" {
		target.Email = previous.Email
	}
	if target.Phone == "" {
		target.Phone = previous.Phone
	}
	if target.BaseRotaryClub == "" && target.RotaryClub == "" {
		target.RotaryClub = previous.RotaryClub
		target.BaseRotaryClub = previous.BaseRotaryClub
		target.BuddyGroup = previous.BuddyGroup
		target.CustomClub = previous.CustomClub
	}
	if target.InvitedBy == "" {
		target.InvitedBy = previous.InvitedBy
	}
	if target.Classification == "" {
		target.Classification = previous.Classification
	}
}

func splitLookupContact(query, email, phone string) (string, string) {
	email = normaliseEmail(email)
	phone = normalisePhone(phone)
	query = cleanText(query)

	if email == "" && phone == "" && query != "" {
		if strings.Contains(query, "@") {
			email = normaliseEmail(query)
		} else {
			phone = normalisePhone(query)
		}
	}

	return email, phone
}

func eventNameForPurpose(purpose string) string {
	switch purpose {
	case "Club Fellowship":
		return "Rotary Club of Kitende Breeze Thursday Fellowship"
	case "Installation":
		return "Rotary Club of Kitende Breeze Presidential Installation"
	case "Service Project":
		return "Rotary Club of Kitende Breeze Service Project"
	default:
		return "Rotary Club of Kitende Breeze Event"
	}
}

func splitClubAndBuddyGroup(value string) (string, string) {
	parts := strings.SplitN(cleanText(value), "|", 2)
	club := ""
	group := ""

	if len(parts) > 0 {
		club = cleanText(parts[0])
	}
	if len(parts) == 2 {
		group = cleanText(parts[1])
	}

	return club, group
}

func normaliseClubName(value string) string {
	club := cleanText(value)
	if club == "" {
		return ""
	}
	if strings.EqualFold(club, nonMember) {
		return nonMember
	}

	const prefix = "Rotary Club of"
	if strings.EqualFold(club, prefix) {
		return ""
	}
	if len(club) > len(prefix) && strings.EqualFold(club[:len(prefix)], prefix) {
		return prefix + " " + cleanText(club[len(prefix):])
	}

	return prefix + " " + club
}

func combineClubAndBuddyGroup(club, buddyGroup string) string {
	club = cleanText(club)
	buddyGroup = strings.ToUpper(cleanText(buddyGroup))
	if club == "" {
		return ""
	}
	if buddyGroup == "" || strings.EqualFold(club, nonMember) {
		return club
	}
	return club + " | " + buddyGroup
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normaliseEmail(value string) string {
	return strings.ToLower(cleanText(value))
}

func normalisePhone(value string) string {
	digits := nonDigitRegex.ReplaceAllString(value, "")
	if digits == "" {
		return ""
	}

	for strings.HasPrefix(digits, "00") {
		digits = strings.TrimPrefix(digits, "00")
	}

	if strings.HasPrefix(digits, "0") {
		digits = strings.TrimLeft(digits, "0")
	}

	if len(digits) == 9 {
		digits = "256" + digits
	}

	return "+" + digits
}

func containsControlCharacter(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func ugandaLocation() *time.Location {
	location, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return location
}

type leaderboardAccumulator struct {
	displayName string
	attendance  int
	people      map[string]struct{}
}

func buildAttendanceSummary(records []models.RegisterRecord) models.AttendanceSummary {
	summary := models.AttendanceSummary{
		TotalAttendance: len(records),
		TopInviters:     make([]models.InviterLeaderboardEntry, 0),
		BuddyGroups:     make([]models.AttendanceLeaderboardEntry, 0),
		Clubs:           make([]models.AttendanceLeaderboardEntry, 0),
	}

	uniqueVisitors := make(map[string]struct{})
	inviters := make(map[string]*leaderboardAccumulator)
	groups := make(map[string]*leaderboardAccumulator)
	clubs := make(map[string]*leaderboardAccumulator)

	for index := range records {
		record := records[index]
		normaliseRecord(&record)
		identity := visitorIdentity(record)
		uniqueVisitors[identity] = struct{}{}

		if record.CustomClub {
			summary.CustomClubAttendance++
		}

		if record.InvitedBy != "" {
			summary.ReferredAttendance++
			addLeaderboardValue(inviters, record.InvitedBy, identity)
		}

		if record.BuddyGroup != "" {
			summary.BuddyGroupAttendance++
			addLeaderboardValue(groups, record.BuddyGroup, identity)
		}

		if record.BaseRotaryClub != "" && !strings.EqualFold(record.BaseRotaryClub, nonMember) {
			addLeaderboardValue(clubs, record.BaseRotaryClub, identity)
		}
	}

	summary.UniqueVisitors = len(uniqueVisitors)
	summary.TopInviters = inviterEntries(inviters)
	summary.BuddyGroups = attendanceEntries(groups)
	summary.Clubs = attendanceEntries(clubs)
	return summary
}

func visitorIdentity(record models.RegisterRecord) string {
	if record.Email != "" {
		return "email:" + strings.ToLower(record.Email)
	}
	if record.Phone != "" {
		return "phone:" + record.Phone
	}
	return "name:" + strings.ToLower(record.FullName)
}

func addLeaderboardValue(target map[string]*leaderboardAccumulator, name, identity string) {
	name = cleanText(name)
	if name == "" {
		return
	}

	key := strings.ToLower(name)
	entry, exists := target[key]
	if !exists {
		entry = &leaderboardAccumulator{
			displayName: name,
			people:      make(map[string]struct{}),
		}
		target[key] = entry
	}

	entry.attendance++
	entry.people[identity] = struct{}{}
}

func inviterEntries(values map[string]*leaderboardAccumulator) []models.InviterLeaderboardEntry {
	entries := make([]models.InviterLeaderboardEntry, 0, len(values))
	for _, value := range values {
		entries = append(entries, models.InviterLeaderboardEntry{
			Name:              value.displayName,
			PeopleConvinced:   len(value.people),
			AttendanceCredits: value.attendance,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].PeopleConvinced != entries[j].PeopleConvinced {
			return entries[i].PeopleConvinced > entries[j].PeopleConvinced
		}
		if entries[i].AttendanceCredits != entries[j].AttendanceCredits {
			return entries[i].AttendanceCredits > entries[j].AttendanceCredits
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

func attendanceEntries(values map[string]*leaderboardAccumulator) []models.AttendanceLeaderboardEntry {
	entries := make([]models.AttendanceLeaderboardEntry, 0, len(values))
	for _, value := range values {
		entries = append(entries, models.AttendanceLeaderboardEntry{
			Name:            value.displayName,
			UniquePeople:    len(value.people),
			AttendanceCount: value.attendance,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].AttendanceCount != entries[j].AttendanceCount {
			return entries[i].AttendanceCount > entries[j].AttendanceCount
		}
		if entries[i].UniquePeople != entries[j].UniquePeople {
			return entries[i].UniquePeople > entries[j].UniquePeople
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

// sendConfirmationAsync runs after the HTTP response has already been
// (or is about to be) written, so it uses its own background context
// with a timeout rather than the request's context, which gets
// cancelled as soon as gin finishes writing the response.
func sendConfirmationAsync(visitorID uint, email, fullName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	registrationID := fmt.Sprintf("KB-2026-%05d", visitorID)

	if err := mail.SendRegistrationConfirmation(
		ctx,
		email,
		fullName,
		eventTime,
		registrationID,
		eventDetailsURL,
		contactPhone,
	); err != nil {
		log.Printf("mail: failed to send registration confirmation to %s (%s): %v", email, registrationID, err)
	}
}
