package models

import (
	"time"

	"gorm.io/gorm"
)

// ClubMember is the club roster. Attendance can automatically refresh member
// details while admins can also maintain the roster directly.
type ClubMember struct {
	gorm.Model
	FullName       string     `gorm:"size:255;not null;index" json:"fullName"`
	Phone          string     `gorm:"size:32;index" json:"phone"`
	Email          string     `gorm:"size:255;index" json:"email"`
	RotaryClub     string     `gorm:"size:255;index" json:"rotaryClub"`
	BuddyGroup     string     `gorm:"size:120;index" json:"buddyGroup"`
	Classification string     `gorm:"size:255" json:"classification"`
	JoinedAt       *time.Time `json:"joinedAt"`
	LastSeenAt     *time.Time `json:"lastSeenAt"`
	Active         bool       `gorm:"not null;default:true;index" json:"active"`
	Source         string     `gorm:"size:50" json:"source"`
}

// Donation records money contributed at a fellowship/event.
type Donation struct {
	gorm.Model
	AttendanceDate string  `gorm:"size:20;index;not null" json:"attendanceDate"`
	DonorName      string  `gorm:"size:255;not null;index" json:"donorName"`
	DonorEmail     string  `gorm:"size:255;index" json:"donorEmail"`
	DonorPhone     string  `gorm:"size:32;index" json:"donorPhone"`
	MemberID       *uint   `gorm:"index" json:"memberId"`
	Amount         float64 `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency       string  `gorm:"size:8;not null;default:UGX" json:"currency"`
	PaymentMethod  string  `gorm:"size:50" json:"paymentMethod"`
	Reference      string  `gorm:"size:120" json:"reference"`
	Notes          string  `gorm:"type:text" json:"notes"`
	RecordedBy     string  `gorm:"size:120" json:"recordedBy"`
}

// ClubGoal tracks measurable club goals against a time line.
type ClubGoal struct {
	gorm.Model
	Title        string  `gorm:"size:255;not null" json:"title"`
	Description  string  `gorm:"type:text" json:"description"`
	Metric       string  `gorm:"size:120" json:"metric"`
	TargetValue  float64 `gorm:"type:numeric(18,2)" json:"targetValue"`
	CurrentValue float64 `gorm:"type:numeric(18,2)" json:"currentValue"`
	Unit         string  `gorm:"size:50" json:"unit"`
	StartDate    string  `gorm:"size:20;index" json:"startDate"`
	DueDate      string  `gorm:"size:20;index" json:"dueDate"`
	Status       string  `gorm:"size:30;index;default:active" json:"status"`
}

// RotaryProject represents a service/fundraising/club project.
type RotaryProject struct {
	gorm.Model
	Name        string  `gorm:"size:255;not null;index" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	Status      string  `gorm:"size:30;index;default:planned" json:"status"`
	StartDate   string  `gorm:"size:20;index" json:"startDate"`
	EndDate     string  `gorm:"size:20;index" json:"endDate"`
	Budget      float64 `gorm:"type:numeric(18,2)" json:"budget"`
	Currency    string  `gorm:"size:8;default:UGX" json:"currency"`
	GoalID      *uint   `gorm:"index" json:"goalId"`
}

// ProjectTransaction records income and expenses against a Rotary project.
type ProjectTransaction struct {
	gorm.Model
	ProjectID       uint    `gorm:"not null;index" json:"projectId"`
	Type            string  `gorm:"size:20;not null;index" json:"type"` // income | expense
	Amount          float64 `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency        string  `gorm:"size:8;default:UGX" json:"currency"`
	Category        string  `gorm:"size:120;index" json:"category"`
	Description     string  `gorm:"type:text" json:"description"`
	TransactionDate string  `gorm:"size:20;index" json:"transactionDate"`
	Reference       string  `gorm:"size:120" json:"reference"`
	RecordedBy      string  `gorm:"size:120" json:"recordedBy"`
}

// ProjectInvoice stores invoice metadata. The file may live in any external
// storage provider and is referenced through FileURL.
type ProjectInvoice struct {
	gorm.Model
	ProjectID     uint       `gorm:"not null;index" json:"projectId"`
	InvoiceNumber string     `gorm:"size:120;not null;index" json:"invoiceNumber"`
	Vendor        string     `gorm:"size:255" json:"vendor"`
	Customer      string     `gorm:"size:255" json:"customer"`
	Description   string     `gorm:"type:text" json:"description"`
	Amount        float64    `gorm:"type:numeric(18,2);not null" json:"amount"`
	Currency      string     `gorm:"size:8;default:UGX" json:"currency"`
	Status        string     `gorm:"size:30;index;default:unpaid" json:"status"`
	IssueDate     string     `gorm:"size:20;index" json:"issueDate"`
	DueDate       string     `gorm:"size:20;index" json:"dueDate"`
	PaidAt        *time.Time `json:"paidAt"`
	FileURL       string     `gorm:"type:text" json:"fileUrl"`
}

// EmailCampaign is an admin-created bulk message/reminder. Processing is done
// asynchronously by Go jobs; HTTP requests only enqueue work.
type EmailCampaign struct {
	gorm.Model
	Name           string     `gorm:"size:255;not null" json:"name"`
	Audience       string     `gorm:"size:50;not null;index" json:"audience"` // members | all_attendees | visitors | attendance_date
	Subject        string     `gorm:"size:255;not null" json:"subject"`
	Body           string     `gorm:"type:text;not null" json:"body"`
	AttendanceDate string     `gorm:"size:20;index" json:"attendanceDate"`
	ScheduledAt    time.Time  `gorm:"index" json:"scheduledAt"`
	Status         string     `gorm:"size:30;index;default:queued" json:"status"`
	CreatedBy      string     `gorm:"size:120" json:"createdBy"`
	SentCount      int        `json:"sentCount"`
	FailedCount    int        `json:"failedCount"`
	CompletedAt    *time.Time `json:"completedAt"`
}

// EmailJob is the durable unit of mail work. DedupeKey makes recurring jobs
// idempotent, including next-day attendance thank-yous.
type EmailJob struct {
	gorm.Model
	CampaignID     *uint      `gorm:"index" json:"campaignId"`
	JobType        string     `gorm:"size:60;not null;index" json:"jobType"`
	DedupeKey      string     `gorm:"size:255;uniqueIndex" json:"dedupeKey"`
	RecipientName  string     `gorm:"size:255" json:"recipientName"`
	RecipientEmail string     `gorm:"size:255;not null;index" json:"recipientEmail"`
	Subject        string     `gorm:"size:255;not null" json:"subject"`
	Body           string     `gorm:"type:text;not null" json:"body"`
	ScheduledAt    time.Time  `gorm:"not null;index" json:"scheduledAt"`
	Status         string     `gorm:"size:30;not null;default:pending;index" json:"status"`
	Attempts       int        `gorm:"not null;default:0" json:"attempts"`
	LastError      string     `gorm:"type:text" json:"lastError"`
	SentAt         *time.Time `json:"sentAt"`
}

type PersonAttendanceLeaderboardEntry struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	RotaryClub      string `json:"rotaryClub"`
	BuddyGroup      string `json:"buddyGroup"`
	AttendanceCount int    `json:"attendanceCount"`
	LastAttendance  string `json:"lastAttendance"`
}

type OperationsDashboard struct {
	TotalMembers      int                                `json:"totalMembers"`
	ActiveMembers     int                                `json:"activeMembers"`
	TotalAttendance   int                                `json:"totalAttendance"`
	UniqueAttendees   int                                `json:"uniqueAttendees"`
	VisitorAttendance int                                `json:"visitorAttendance"`
	TotalDonations    float64                            `json:"totalDonations"`
	DonationCurrency  string                             `json:"donationCurrency"`
	ActiveGoals       int                                `json:"activeGoals"`
	ActiveProjects    int                                `json:"activeProjects"`
	PendingMailJobs   int                                `json:"pendingMailJobs"`
	TopAttendees      []PersonAttendanceLeaderboardEntry `json:"topAttendees"`
	BuddyGroups       []AttendanceLeaderboardEntry       `json:"buddyGroups"`
}

type ProjectSummary struct {
	Project      RotaryProject        `json:"project"`
	Income       float64              `json:"income"`
	Expenses     float64              `json:"expenses"`
	Balance      float64              `json:"balance"`
	Transactions []ProjectTransaction `json:"transactions,omitempty"`
	Invoices     []ProjectInvoice     `json:"invoices,omitempty"`
}
