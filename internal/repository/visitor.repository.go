package repository

import "github.com/wksn753/kitende-rotary/internal/models"

type VisitorRepository interface {
	SaveVisitorRecord(record *models.RegisterRecord) error
	GetVisitorRecords() ([]models.RegisterRecord, error)
	GetVisitorRecordsByDate(attendanceDate string) ([]models.RegisterRecord, error)
	FindReturningVisitor(email, phone string) (*models.RegisterRecord, error)
	FindSameDayAttendance(email, phone, fullName, attendanceDate, event, purpose string) (*models.RegisterRecord, error)
	UpsertRotaryClub(name string, isCustom bool) error
	GetRotaryClubs() ([]models.RotaryClub, error)
}
