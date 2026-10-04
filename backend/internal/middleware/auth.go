package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"villageconnect/internal/auth"
	"villageconnect/internal/models"
	"villageconnect/internal/services"
)

const (
	UserIDKey   = "auth.userID"
	RoleKey     = "auth.role"
	ClaimsKey   = "auth.claims"
	AuthUserKey = "auth.user"
)

type SessionValidator interface {
	ValidateSession(context.Context, *auth.Claims) (models.User, error)
}

func RequireAuth(secret string, validator SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := auth.ValidateToken(secret, c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			return
		}
		user, err := validator.ValidateSession(c.Request.Context(), claims)
		if err != nil {
			if errors.Is(err, services.ErrUnauthorized) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "Session is invalid or expired",
				})
				return
			}
			log.Printf("authentication session validation failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Unable to validate session",
			})
			return
		}
		c.Set(UserIDKey, user.ID)
		c.Set(RoleKey, string(user.Role))
		c.Set(ClaimsKey, claims)
		c.Set(AuthUserKey, user)
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		role, exists := c.Get(RoleKey)
		roleName, validRole := role.(string)
		if !exists || !validRole {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authentication required",
			})
			return
		}
		if _, ok := allowed[roleName]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Insufficient permissions",
			})
			return
		}
		if roleName != string(models.RoleCustomer) {
			userValue, exists := c.Get(AuthUserKey)
			user, validUser := userValue.(models.User)
			if !exists || !validUser {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"message": "Authentication required",
				})
				return
			}
			if !user.Verified {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"message": "Account verification is required",
				})
				return
			}
		}
		c.Next()
	}
}

func RequireCustomer() gin.HandlerFunc {
	return RequireRoles(string(models.RoleCustomer))
}
func RequireSeller() gin.HandlerFunc {
	return RequireRoles(string(models.RoleSeller))
}
func RequireDeliveryAgent() gin.HandlerFunc {
	return RequireRoles(string(models.RoleDeliveryAgent))
}
func RequireAdmin() gin.HandlerFunc {
	return RequireRoles(string(models.RoleAdmin))
}

func Identity(c *gin.Context) (string, string) {
	userID, _ := c.Get(UserIDKey)
	role, _ := c.Get(RoleKey)
	id, _ := userID.(string)
	roleName, _ := role.(string)
	return id, roleName
}

func AuthenticatedClaims(c *gin.Context) (*auth.Claims, bool) {
	value, exists := c.Get(ClaimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := value.(*auth.Claims)
	return claims, ok
}
