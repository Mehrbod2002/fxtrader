package middleware

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/mehrbod2002/fxtrader/internal/config"
	"github.com/mehrbod2002/fxtrader/internal/service"

	"github.com/gin-gonic/gin"
)

func UserAuthMiddleware(userService service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		return
		telegramID := c.GetHeader("X-Telegram-ID")
		if telegramID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-Telegram-ID header required"})
			return
		}

		user, err := userService.GetUserByTelegramID(telegramID)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Telegram ID"})
			return
		}

		if user.AccountType == "admin" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Admin accounts cannot use user routes"})
			return
		}

		c.Set("user_id", user.ID.Hex())
		c.Next()
	}
}

func GenerateAdminJWT(userID string, cfg *config.Config) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  userID,
		"is_admin": true,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	return token.SignedString([]byte(cfg.JWTSecret))
}
