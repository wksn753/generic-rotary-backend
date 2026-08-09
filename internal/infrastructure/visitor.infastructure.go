package infrastructure

import (
	"strings"

	"github.com/wksn753/kitende-rotary/internal/models"
	"github.com/wksn753/kitende-rotary/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VisitorInfrastructure struct {
	db *gorm.DB
}

func NewVisitorInfrastructure(db *gorm.DB) repository.VisitorRepository {
	return &VisitorInfrastructure{db: db}
}

func (v *VisitorInfrastructure) SaveVisitorRecord(record *models.RegisterRecord) error {
	return v.db.Create(record).Error
}

func (v *VisitorInfrastructure) GetVisitorRecords() ([]models.RegisterRecord, error) {
	var records []models.RegisterRecord
	err := v.db.Order("attendance_date DESC, created_at DESC").Find(&records).Error
	return records, err
}

func (v *VisitorInfrastructure) GetVisitorRecordsByDate(attendanceDate string) ([]models.RegisterRecord, error) {
	var records []models.RegisterRecord
	query := v.db.Order("created_at DESC")

	if strings.TrimSpace(attendanceDate) != "" {
		query = query.Where("attendance_date = ?", strings.TrimSpace(attendanceDate))
	}

	err := query.Find(&records).Error
	return records, err
}

// FindReturningVisitor finds the most recent historical profile for a contact.
// It intentionally does not treat the person as a duplicate because each
// fellowship/event attendance needs its own check-in row.
func (v *VisitorInfrastructure) FindReturningVisitor(email, phone string) (*models.RegisterRecord, error) {
	var record models.RegisterRecord

	query := v.db.Order("created_at DESC")
	clauses, args := identityClauses(email, phone, "", false)
	if len(clauses) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	err := query.Where("("+strings.Join(clauses, " OR ")+")", args...).First(&record).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// FindSameDayAttendance only blocks repeated check-in for the same day/event.
// It allows the same guest to attend every Thursday without being rejected.
func (v *VisitorInfrastructure) FindSameDayAttendance(email, phone, fullName, attendanceDate, event, purpose string) (*models.RegisterRecord, error) {
	var record models.RegisterRecord

	query := v.db.Where("attendance_date = ?", attendanceDate)

	if event != "" {
		query = query.Where("event = ?", event)
	}

	if purpose != "" {
		query = query.Where("purpose = ?", purpose)
	}

	clauses, args := identityClauses(email, phone, fullName, true)
	if len(clauses) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	err := query.Where("("+strings.Join(clauses, " OR ")+")", args...).First(&record).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// UpsertRotaryClub persists a club added through registration. NormalizedName
// provides a case-insensitive identity so variants do not create duplicates.
func (v *VisitorInfrastructure) UpsertRotaryClub(name string, isCustom bool) error {
	name = strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if name == "" {
		return nil
	}

	club := models.RotaryClub{
		Name:           name,
		NormalizedName: strings.ToLower(name),
		IsCustom:       isCustom,
		Active:         true,
	}

	return v.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "normalized_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"name":       name,
			"active":     true,
			"is_custom":  gorm.Expr("rotary_clubs.is_custom OR EXCLUDED.is_custom"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(&club).Error
}

func (v *VisitorInfrastructure) GetRotaryClubs() ([]models.RotaryClub, error) {
	var clubs []models.RotaryClub
	err := v.db.Where("active = ?", true).Order("name ASC").Find(&clubs).Error
	return clubs, err
}

func identityClauses(email, phone, fullName string, includeNameFallback bool) ([]string, []interface{}) {
	clauses := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)

	if strings.TrimSpace(email) != "" {
		clauses = append(clauses, "LOWER(email) = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(email)))
	}

	if strings.TrimSpace(phone) != "" {
		clauses = append(clauses, "phone = ?")
		args = append(args, strings.TrimSpace(phone))
	}

	// Fallback for walk-in guests who refuse to leave email/phone.
	if includeNameFallback && len(clauses) == 0 && strings.TrimSpace(fullName) != "" {
		clauses = append(clauses, "LOWER(full_name) = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(fullName)))
	}

	return clauses, args
}
