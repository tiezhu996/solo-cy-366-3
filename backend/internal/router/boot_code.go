package router

import (
	"github.com/gin-gonic/gin"

	"github.com/esportsbar/backend/internal/constants"
	"github.com/esportsbar/backend/internal/handler"
	"github.com/esportsbar/backend/internal/middleware"
)

// RegisterBootCode 注册动态开机码路由。
//   - POST /boot-codes、GET /boot-codes/mine：会员生成/查看自己的动态开机码
//   - POST /boot-codes/verify、GET /boot-codes：店员/管理员扫码核销与核销记录
func RegisterBootCode(rg *gin.RouterGroup, h *handler.BootCodeHandler, jwtSecret string) {
	bootCodes := rg.Group("/boot-codes", middleware.Auth(jwtSecret))
	{
		// 会员：生成动态开机码
		bootCodes.POST("", middleware.RBAC(constants.RoleMember), h.Generate)
		// 会员：查询当前有效开机码
		bootCodes.GET("/mine", middleware.RBAC(constants.RoleMember), h.Mine)
		// 店员/管理员：扫码校验并开机
		bootCodes.POST("/verify", middleware.RBAC(constants.RoleAdmin, constants.RoleStaff), h.Verify)
		// 店员/管理员：核销记录
		bootCodes.GET("", middleware.RBAC(constants.RoleAdmin, constants.RoleStaff), h.List)
	}
}
