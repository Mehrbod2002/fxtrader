package middleware

import (
	"fxtrader/internal/config"
	"fxtrader/internal/service"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

func UserAuthMiddleware(userService service.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		telegramID := c.GetHeader("X-Telegram-ID")
		if telegramID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Telegram-ID header required"})
			c.Abort()
			return
		}

		user, err := userService.GetUserByTelegramID(telegramID)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Telegram ID"})
			c.Abort()
			return
		}

		if user.AccountType == "admin" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin accounts cannot use user routes"})
			c.Abort()
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
