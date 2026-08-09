package models

import (
	"time"

	"gorm.io/gorm"
)

// RegisterRecord is an attendance/check-in row.
//
// One person may have many RegisterRecord rows because Kitende Breeze has
// weekly Thursday fellowship plus one-off events. Returning guests are matched
// by email/phone, their previous details are reused, and a fresh row is saved
// for the selected attendance date/event.
type RegisterRecord struct {
	gorm.Model
	FullName       string    `gorm:"size:255;not null" json:"fullName"`
	Phone          string    `gorm:"size:32;index" json:"phone"`
	Email          string    `gorm:"size:255;index" json:"email"`
	RotaryClub     string    `gorm:"size:380" json:"rotaryClub"`
	BaseRotaryClub string    `gorm:"size:255;index" json:"baseRotaryClub"`
	BuddyGroup     string    `gorm:"size:120;index" json:"buddyGroup"`
	InvitedBy      string    `gorm:"size:255;index" json:"invitedBy"`
	CustomClub     bool      `gorm:"not null;default:false;index" json:"customClub"`
	Classification string    `gorm:"size:255" json:"classification"`
	Purpose        string    `gorm:"size:255;index" json:"purpose"`
	OtherPurpose   string    `gorm:"type:text" json:"otherPurpose"`
	Event          string    `gorm:"size:255;index" json:"event"`
	EventDate      string    `gorm:"size:100" json:"date"`
	AttendanceDate string    `gorm:"size:20;index" json:"attendanceDate"`
	Venue          string    `gorm:"size:255" json:"venue"`
	CheckInSource  string    `gorm:"size:50" json:"checkInSource"`
	SubmittedAt    time.Time `json:"submittedAt"`
}

// ReturningVisitor is the reusable profile returned by the lookup endpoint.
type ReturningVisitor struct {
	ID             uint   `json:"id"`
	FullName       string `json:"fullName"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	RotaryClub     string `json:"rotaryClub"`
	BaseRotaryClub string `json:"baseRotaryClub"`
	BuddyGroup     string `json:"buddyGroup"`
	InvitedBy      string `json:"invitedBy"`
	CustomClub     bool   `json:"customClub"`
	Classification string `json:"classification"`
}

// RotaryClub stores clubs added through registration so they can be returned
// by the public club-directory endpoint and reused by future visitors.
type RotaryClub struct {
	gorm.Model
	Name           string `gorm:"size:255;not null" json:"name"`
	NormalizedName string `gorm:"size:255;not null;uniqueIndex" json:"-"`
	IsCustom       bool   `gorm:"not null;default:true" json:"isCustom"`
	Active         bool   `gorm:"not null;default:true;index" json:"active"`
}

// RotaryClubOption is the public club-directory representation.
type RotaryClubOption struct {
	Name     string `json:"name"`
	IsCustom bool   `json:"isCustom"`
}

// InviterLeaderboardEntry reports unique people convinced and all attendance
// credits. Unique people prevents the same returning visitor being counted as
// a new person every Thursday.
type InviterLeaderboardEntry struct {
	Name              string `json:"name"`
	PeopleConvinced   int    `json:"peopleConvinced"`
	AttendanceCredits int    `json:"attendanceCredits"`
}

// AttendanceLeaderboardEntry is used for buddy-group and Rotary-club totals.
type AttendanceLeaderboardEntry struct {
	Name            string `json:"name"`
	UniquePeople    int    `json:"uniquePeople"`
	AttendanceCount int    `json:"attendanceCount"`
}

// AttendanceSummary contains dashboard-ready totals and leaderboards.
type AttendanceSummary struct {
	TotalAttendance      int                          `json:"totalAttendance"`
	UniqueVisitors       int                          `json:"uniqueVisitors"`
	CustomClubAttendance int                          `json:"customClubAttendance"`
	BuddyGroupAttendance int                          `json:"buddyGroupAttendance"`
	ReferredAttendance   int                          `json:"referredAttendance"`
	TopInviters          []InviterLeaderboardEntry    `json:"topInviters"`
	BuddyGroups          []AttendanceLeaderboardEntry `json:"buddyGroups"`
	Clubs                []AttendanceLeaderboardEntry `json:"clubs"`
}
