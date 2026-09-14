package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wksn753/kitende-rotary/internal/models"
	"github.com/wksn753/kitende-rotary/internal/operations"
	"gorm.io/gorm"
)

type OperationsHandler struct {
	service *operations.Service
	db      *gorm.DB
}

func NewOperationsHandler(service *operations.Service) *OperationsHandler {
	return &OperationsHandler{service: service, db: service.DB()}
}

func (h *OperationsHandler) Dashboard(c *gin.Context) {
	dashboard, err := h.service.Dashboard()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load operations dashboard"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "dashboard": dashboard})
}

func (h *OperationsHandler) ListMembers(c *gin.Context) {
	var rows []models.ClubMember
	query := h.db.Order("active DESC, full_name ASC")
	if strings.EqualFold(c.Query("active"), "true") {
		query = query.Where("active = ?", true)
	}
	if err := query.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "members": rows, "count": len(rows)})
}

func (h *OperationsHandler) CreateMember(c *gin.Context) {
	var row models.ClubMember
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid member payload"})
		return
	}
	row.FullName = strings.TrimSpace(row.FullName)
	row.Email = strings.ToLower(strings.TrimSpace(row.Email))
	row.Phone = strings.TrimSpace(row.Phone)
	if row.FullName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Member name is required"})
		return
	}
	if row.Source == "" {
		row.Source = "admin"
	}
	row.Active = true
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create member"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "member": row})
}

func (h *OperationsHandler) UpdateMember(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var row models.ClubMember
	if err := h.db.First(&row, id).Error; err != nil {
		notFound(c, "Member")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid member payload"})
		return
	}
	allowed := map[string]bool{"full_name": true, "phone": true, "email": true, "rotary_club": true, "buddy_group": true, "classification": true, "joined_at": true, "active": true}
	updates := filterUpdates(payload, allowed)
	if email, ok := updates["email"].(string); ok {
		updates["email"] = strings.ToLower(strings.TrimSpace(email))
	}
	if err := h.db.Model(&row).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update member"})
		return
	}
	h.db.First(&row, id)
	c.JSON(http.StatusOK, gin.H{"success": true, "member": row})
}

func (h *OperationsHandler) ListDonations(c *gin.Context) {
	var rows []models.Donation
	query := h.db.Order("attendance_date DESC, created_at DESC")
	if date := strings.TrimSpace(c.Query("date")); date != "" {
		query = query.Where("attendance_date = ?", date)
	}
	if err := query.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load donations"})
		return
	}
	var total struct{ Total float64 }
	base := h.db.Model(&models.Donation{})
	if date := strings.TrimSpace(c.Query("date")); date != "" {
		base = base.Where("attendance_date = ?", date)
	}
	_ = base.Select("COALESCE(SUM(amount),0) AS total").Scan(&total).Error
	c.JSON(http.StatusOK, gin.H{"success": true, "donations": rows, "total": total.Total, "count": len(rows)})
}

func (h *OperationsHandler) CreateDonation(c *gin.Context) {
	var row models.Donation
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid donation payload"})
		return
	}
	if strings.TrimSpace(row.DonorName) == "" || row.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Donor name and an amount greater than zero are required"})
		return
	}
	if row.AttendanceDate == "" {
		row.AttendanceDate = time.Now().In(kampalaLocation()).Format("2006-01-02")
	}
	if row.Currency == "" {
		row.Currency = "UGX"
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to record donation"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "donation": row})
}

func (h *OperationsHandler) DeleteDonation(c *gin.Context) {
	h.deleteByID(c, &models.Donation{}, "Donation")
}

func (h *OperationsHandler) ListGoals(c *gin.Context) {
	var rows []models.ClubGoal
	if err := h.db.Order("due_date ASC, created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load goals"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "goals": rows, "count": len(rows)})
}

func (h *OperationsHandler) CreateGoal(c *gin.Context) {
	var row models.ClubGoal
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid goal payload"})
		return
	}
	row.Title = strings.TrimSpace(row.Title)
	if row.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Goal title is required"})
		return
	}
	if row.Status == "" {
		row.Status = "active"
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create goal"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "goal": row})
}

func (h *OperationsHandler) UpdateGoal(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var row models.ClubGoal
	if err := h.db.First(&row, id).Error; err != nil {
		notFound(c, "Goal")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid goal payload"})
		return
	}
	allowed := map[string]bool{"title": true, "description": true, "metric": true, "target_value": true, "current_value": true, "unit": true, "start_date": true, "due_date": true, "status": true}
	if err := h.db.Model(&row).Updates(filterUpdates(payload, allowed)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update goal"})
		return
	}
	h.db.First(&row, id)
	c.JSON(http.StatusOK, gin.H{"success": true, "goal": row})
}
func (h *OperationsHandler) DeleteGoal(c *gin.Context) { h.deleteByID(c, &models.ClubGoal{}, "Goal") }

func (h *OperationsHandler) ListProjects(c *gin.Context) {
	var projects []models.RotaryProject
	if err := h.db.Order("created_at DESC").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load projects"})
		return
	}
	summaries := make([]models.ProjectSummary, 0, len(projects))
	for _, project := range projects {
		summary, err := h.projectSummary(project, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to summarize projects"})
			return
		}
		summaries = append(summaries, summary)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "projects": summaries, "count": len(summaries)})
}

func (h *OperationsHandler) CreateProject(c *gin.Context) {
	var row models.RotaryProject
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid project payload"})
		return
	}
	row.Name = strings.TrimSpace(row.Name)
	if row.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Project name is required"})
		return
	}
	if row.Status == "" {
		row.Status = "planned"
	}
	if row.Currency == "" {
		row.Currency = "UGX"
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create project"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "project": row})
}

func (h *OperationsHandler) GetProject(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var row models.RotaryProject
	if err := h.db.First(&row, id).Error; err != nil {
		notFound(c, "Project")
		return
	}
	summary, err := h.projectSummary(row, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load project"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "project": summary})
}

func (h *OperationsHandler) UpdateProject(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var row models.RotaryProject
	if err := h.db.First(&row, id).Error; err != nil {
		notFound(c, "Project")
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid project payload"})
		return
	}
	allowed := map[string]bool{"name": true, "description": true, "status": true, "start_date": true, "end_date": true, "budget": true, "currency": true, "goal_id": true}
	if err := h.db.Model(&row).Updates(filterUpdates(payload, allowed)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to update project"})
		return
	}
	h.db.First(&row, id)
	c.JSON(http.StatusOK, gin.H{"success": true, "project": row})
}
func (h *OperationsHandler) DeleteProject(c *gin.Context) {
	h.deleteByID(c, &models.RotaryProject{}, "Project")
}

func (h *OperationsHandler) CreateProjectTransaction(c *gin.Context) {
	projectID, ok := parseID(c)
	if !ok {
		return
	}
	var project models.RotaryProject
	if err := h.db.First(&project, projectID).Error; err != nil {
		notFound(c, "Project")
		return
	}
	var row models.ProjectTransaction
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid transaction payload"})
		return
	}
	row.ProjectID = projectID
	row.Type = strings.ToLower(strings.TrimSpace(row.Type))
	if row.Type != "income" && row.Type != "expense" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Transaction type must be income or expense"})
		return
	}
	if row.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Transaction amount must be greater than zero"})
		return
	}
	if row.Currency == "" {
		row.Currency = project.Currency
	}
	if row.Currency == "" {
		row.Currency = "UGX"
	}
	if row.TransactionDate == "" {
		row.TransactionDate = time.Now().In(kampalaLocation()).Format("2006-01-02")
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to record transaction"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "transaction": row})
}

func (h *OperationsHandler) CreateProjectInvoice(c *gin.Context) {
	projectID, ok := parseID(c)
	if !ok {
		return
	}
	var project models.RotaryProject
	if err := h.db.First(&project, projectID).Error; err != nil {
		notFound(c, "Project")
		return
	}
	var row models.ProjectInvoice
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid invoice payload"})
		return
	}
	row.ProjectID = projectID
	row.InvoiceNumber = strings.TrimSpace(row.InvoiceNumber)
	if row.InvoiceNumber == "" || row.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invoice number and amount are required"})
		return
	}
	if row.Currency == "" {
		row.Currency = project.Currency
	}
	if row.Currency == "" {
		row.Currency = "UGX"
	}
	if row.Status == "" {
		row.Status = "unpaid"
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to record invoice"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "invoice": row})
}

func (h *OperationsHandler) ListCampaigns(c *gin.Context) {
	var rows []models.EmailCampaign
	if err := h.db.Order("created_at DESC").Limit(100).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load campaigns"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "campaigns": rows, "count": len(rows)})
}

func (h *OperationsHandler) CreateCampaign(c *gin.Context) {
	var row models.EmailCampaign
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid campaign payload"})
		return
	}
	if err := h.service.CreateCampaign(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "campaign": row})
}

func (h *OperationsHandler) RunJobs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Second)
	defer cancel()
	sent, failed, err := h.service.RunDueJobs(ctx, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to process mail jobs", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "sent": sent, "failed": failed})
}

func (h *OperationsHandler) projectSummary(project models.RotaryProject, includeDetails bool) (models.ProjectSummary, error) {
	var amounts struct{ Income, Expenses float64 }
	if err := h.db.Model(&models.ProjectTransaction{}).
		Select("COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END),0) AS income, COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END),0) AS expenses").
		Where("project_id = ?", project.ID).Scan(&amounts).Error; err != nil {
		return models.ProjectSummary{}, err
	}
	out := models.ProjectSummary{Project: project, Income: amounts.Income, Expenses: amounts.Expenses, Balance: amounts.Income - amounts.Expenses}
	if includeDetails {
		if err := h.db.Where("project_id = ?", project.ID).Order("transaction_date DESC, created_at DESC").Find(&out.Transactions).Error; err != nil {
			return out, err
		}
		if err := h.db.Where("project_id = ?", project.ID).Order("issue_date DESC, created_at DESC").Find(&out.Invoices).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}

func (h *OperationsHandler) deleteByID(c *gin.Context, model any, label string) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result := h.db.Delete(model, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("Failed to delete %s", strings.ToLower(label))})
		return
	}
	if result.RowsAffected == 0 {
		notFound(c, label)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": label + " deleted"})
}

func parseID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid id"})
		return 0, false
	}
	return uint(value), true
}

func notFound(c *gin.Context, label string) {
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": label + " not found"})
}

// JSON uses camelCase while GORM Updates expects column names. Normalize the
// small set of fields admins are allowed to change.
func filterUpdates(payload map[string]any, allowed map[string]bool) map[string]any {
	updates := map[string]any{}
	for key, value := range payload {
		column := camelToColumn(key)
		if allowed[column] {
			updates[column] = value
		}
	}
	return updates
}

func camelToColumn(value string) string {
	mapping := map[string]string{
		"fullName": "full_name", "rotaryClub": "rotary_club", "buddyGroup": "buddy_group", "joinedAt": "joined_at",
		"targetValue": "target_value", "currentValue": "current_value", "startDate": "start_date", "dueDate": "due_date",
		"endDate": "end_date", "goalId": "goal_id",
	}
	if mapped := mapping[value]; mapped != "" {
		return mapped
	}
	return value
}

func kampalaLocation() *time.Location {
	loc, err := time.LoadLocation("Africa/Kampala")
	if err != nil {
		return time.FixedZone("EAT", 3*60*60)
	}
	return loc
}
