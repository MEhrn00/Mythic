package authentication

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/its-a-feature/Mythic/database"
	"github.com/its-a-feature/Mythic/logging"
	"github.com/its-a-feature/Mythic/utils"
)

func JwtAuthMiddleware() gin.HandlerFunc {
	// verify that all authenticated requests have valid signatures and aren't expired
	return func(c *gin.Context) {
		if err := TokenValid(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func CookieAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := CookieTokenValid(c); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			c.Abort()
			return
		} else {
			c.Next()
		}
	}
}

func IPBlockMiddleware() gin.HandlerFunc {
	// verify that xforward-for address is in range from utils.Config.AllowedIPBlocks
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ipAddr := net.ParseIP(ip)
		if ipAddr == nil {
			logging.LogError(nil, "Failed to parse client IP", "client_ip", ip)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			c.Abort()
			return
		} else {
			// make sure the ipAddr is in at least one of the alowed IP blocks
			for _, subnet := range utils.MythicConfig.AllowedIPBlocks {
				logging.LogTrace("Checking if IP in allowed IP blocks", "client_ip", ipAddr, "current IP Block", subnet)
				if subnet.Contains(ipAddr) {
					c.Next()
					return
				}
			}
		}
		logging.LogError(nil, "Client IP not in allowed IP blocks", "client_ip", ipAddr)
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		c.Abort()
	}
}

func RBACMiddleware(allowedRoles []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if customClaims, err := GetClaims(c); err != nil {
			logging.LogError(err, "Failed to get claims for RBACMiddleware")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			c.Abort()
		} else if operatorOperation, err := database.GetUserCurrentOperation(customClaims.UserID); err != nil {
			logging.LogError(err, "Failed to get user current operation")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			c.Abort()
		} else if operatorOperation.CurrentOperator.Admin || utils.SliceContains(allowedRoles, operatorOperation.ViewMode) {
			c.Set("operatorOperation", operatorOperation)
			c.Next()
			return
		} else {
			logging.LogError(nil, "Unauthorized view mode for operation")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
			c.Abort()
		}
	}
}

func RBACMiddlewareAll() gin.HandlerFunc {
	return RBACMiddleware([]string{
		database.OPERATOR_OPERATION_VIEW_MODE_LEAD,
		database.OPERATOR_OPERATION_VIEW_MODE_OPERATOR,
		database.OPERATOR_OPERATION_VIEW_MODE_SPECTATOR,
	})
}

func RBACMiddlewareNoSpectators() gin.HandlerFunc {
	return RBACMiddleware([]string{
		database.OPERATOR_OPERATION_VIEW_MODE_LEAD,
		database.OPERATOR_OPERATION_VIEW_MODE_OPERATOR,
	})
}

func RBACMiddlewareOperationAdmin() gin.HandlerFunc {
	return RBACMiddleware([]string{
		database.OPERATOR_OPERATION_VIEW_MODE_LEAD,
	})
}