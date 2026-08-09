package handlers

import (
	"testing"

	"github.com/wksn753/kitende-rotary/internal/models"
)

func TestNormaliseRecordBuildsClubAndUppercaseBuddyGroup(t *testing.T) {
	record := models.RegisterRecord{
		FullName:   "  Peter   Kato ",
		Email:      "PETER@EXAMPLE.COM",
		RotaryClub: "Kampala Central | gopus 4",
	}

	normaliseRecord(&record)

	if record.BaseRotaryClub != "Rotary Club of Kampala Central" {
		t.Fatalf("unexpected base club: %q", record.BaseRotaryClub)
	}
	if record.BuddyGroup != "GOPUS 4" {
		t.Fatalf("unexpected buddy group: %q", record.BuddyGroup)
	}
	if record.RotaryClub != "Rotary Club of Kampala Central | GOPUS 4" {
		t.Fatalf("unexpected display club: %q", record.RotaryClub)
	}
	if record.Email != "peter@example.com" {
		t.Fatalf("unexpected email: %q", record.Email)
	}
}

func TestNormaliseRecordUsesStructuredFields(t *testing.T) {
	record := models.RegisterRecord{
		BaseRotaryClub: "Rotary Club of Kitende",
		BuddyGroup:     " group alpha ",
	}

	normaliseRecord(&record)

	if record.RotaryClub != "Rotary Club of Kitende | GROUP ALPHA" {
		t.Fatalf("unexpected display value: %q", record.RotaryClub)
	}
}

func TestNormaliseRecordNonMemberCannotHaveBuddyGroup(t *testing.T) {
	record := models.RegisterRecord{
		BaseRotaryClub: nonMember,
		BuddyGroup:     "GROUP ALPHA",
		CustomClub:     true,
	}

	normaliseRecord(&record)

	if record.BuddyGroup != "" {
		t.Fatalf("non-member buddy group should be empty: %q", record.BuddyGroup)
	}
	if record.CustomClub {
		t.Fatal("non-member must not be marked as a custom club")
	}
	if record.RotaryClub != nonMember {
		t.Fatalf("unexpected non-member display value: %q", record.RotaryClub)
	}
}

func TestAttendanceSummaryCountsUniquePeople(t *testing.T) {
	records := []models.RegisterRecord{
		{
			FullName:       "Alice",
			Email:          "alice@example.com",
			BaseRotaryClub: "Rotary Club of Kitende",
			BuddyGroup:     "GROUP ALPHA",
			InvitedBy:      "John Mugisha",
			CustomClub:     true,
		},
		{
			FullName:       "Alice",
			Email:          "alice@example.com",
			BaseRotaryClub: "Rotary Club of Kitende",
			BuddyGroup:     "GROUP ALPHA",
			InvitedBy:      "John Mugisha",
		},
		{
			FullName:       "Bob",
			Phone:          "+256700000001",
			BaseRotaryClub: "Rotary Club of Kitende",
			BuddyGroup:     "GROUP ALPHA",
			InvitedBy:      "John Mugisha",
		},
		{
			FullName:       "Carol",
			Phone:          "+256700000002",
			BaseRotaryClub: "Rotary Club of Entebbe",
			BuddyGroup:     "GROUP BETA",
			InvitedBy:      "Jane Namusoke",
		},
	}

	summary := buildAttendanceSummary(records)

	if summary.TotalAttendance != 4 {
		t.Fatalf("expected 4 attendance rows, got %d", summary.TotalAttendance)
	}
	if summary.UniqueVisitors != 3 {
		t.Fatalf("expected 3 unique visitors, got %d", summary.UniqueVisitors)
	}
	if summary.CustomClubAttendance != 1 {
		t.Fatalf("expected 1 custom-club attendance, got %d", summary.CustomClubAttendance)
	}
	if len(summary.TopInviters) != 2 {
		t.Fatalf("expected 2 inviter entries, got %d", len(summary.TopInviters))
	}

	top := summary.TopInviters[0]
	if top.Name != "John Mugisha" || top.PeopleConvinced != 2 || top.AttendanceCredits != 3 {
		t.Fatalf("unexpected top inviter: %#v", top)
	}

	if len(summary.BuddyGroups) != 2 {
		t.Fatalf("expected 2 buddy groups, got %d", len(summary.BuddyGroups))
	}
	if summary.BuddyGroups[0].Name != "GROUP ALPHA" || summary.BuddyGroups[0].UniquePeople != 2 || summary.BuddyGroups[0].AttendanceCount != 3 {
		t.Fatalf("unexpected top buddy group: %#v", summary.BuddyGroups[0])
	}
}

func TestValidateRecordRequiresClub(t *testing.T) {
	record := models.RegisterRecord{
		FullName:       "Visitor One",
		Email:          "visitor@example.com",
		AttendanceDate: "2026-07-23",
	}

	if message := validateRecord(&record); message == "" {
		t.Fatal("expected missing-club validation error")
	}
}
