package api

import (
	"fxtrader/internal/models"
	"fxtrader/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RuleHandler struct {
	ruleService service.RuleService
}

func NewRuleHandler(ruleService service.RuleService) *RuleHandler {
	return &RuleHandler{ruleService: ruleService}
}

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

func (h *RuleHandler) GetAllRules(c *gin.Context) {
	rules, err := h.ruleService.GetAllRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve rules"})
		return
	}
	c.JSON(http.StatusOK, rules)
}

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

func (h *RuleHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.ruleService.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "Rule deleted"})
}
