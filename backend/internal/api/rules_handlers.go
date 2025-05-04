package api

import (
	"net/http"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
)

type RuleHandler struct {
	ruleService service.RuleService
}

func NewRuleHandler(ruleService service.RuleService) *RuleHandler {
	return &RuleHandler{ruleService: ruleService}
}

// @Summary Create a new rule
// @Description Adds a new rule to the system (admin only)
// @Tags Rules
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param rule body models.Rule true "Rule data"
// @Success 201 {object} map[string]string "Rule created"
// @Failure 400 {object} map[string]string "Invalid JSON or empty content"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to create rule"
// @Router /admin/rules [post]
func (h *RuleHandler) CreateRule(c *gin.Context) {
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if rule.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule content cannot be empty"})
		return
	}

	if err := h.ruleService.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Rule created", "rule_id": rule.ID.Hex()})
}

// @Summary Get rule by ID
// @Description Retrieves details of a rule by ID (admin only)
// @Tags Rules
// @Produce json
// @Security BasicAuth
// @Param id path string true "Rule ID"
// @Success 200 {object} models.Rule
// @Failure 400 {object} map[string]string "Invalid rule ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 404 {object} map[string]string "Rule not found"
// @Router /admin/rules/{id} [get]
func (h *RuleHandler) GetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.ruleService.GetRule(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// @Summary Get all rules
// @Description Retrieves a list of all rules (accessible to all users)
// @Tags Rules
// @Produce json
// @Success 200 {array} models.Rule
// @Failure 500 {object} map[string]string "Failed to retrieve rules"
// @Router /rules [get]
func (h *RuleHandler) GetAllRules(c *gin.Context) {
	rules, err := h.ruleService.GetAllRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve rules"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

// @Summary Update a rule
// @Description Updates the content of an existing rule (admin only)
// @Tags Rules
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param id path string true "Rule ID"
// @Param rule body models.Rule true "Updated rule data"
// @Success 200 {object} map[string]string "Rule updated"
// @Failure 400 {object} map[string]string "Invalid JSON or empty content"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to update rule"
// @Router /admin/rules/{id} [put]
func (h *RuleHandler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if rule.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule content cannot be empty"})
		return
	}

	if err := h.ruleService.UpdateRule(id, &rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "Rule updated"})
}

// @Summary Delete a rule
// @Description Removes a rule from the system (admin only)
// @Tags Rules
// @Produce json
// @Security BasicAuth
// @Param id path string true "Rule ID"
// @Success 200 {object} map[string]string "Rule deleted"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Failed to delete rule"
// @Router /admin/rules/{id} [delete]
func (h *RuleHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.ruleService.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "Rule deleted"})
}
