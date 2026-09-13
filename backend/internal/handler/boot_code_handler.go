package handler

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/esportsbar/backend/internal/constants"
	"github.com/esportsbar/backend/internal/dto"
	"github.com/esportsbar/backend/internal/model"
	"github.com/esportsbar/backend/internal/service"
	"github.com/esportsbar/backend/internal/util"
	"github.com/esportsbar/backend/pkg/response"
)

// BootCodeHandler 动态开机码接口处理器（会员生成、店员扫码核销）。
type BootCodeHandler struct {
	bootCodeService *service.BootCodeService
	logger          *slog.Logger
}

// NewBootCodeHandler 构造动态开机码处理器。
func NewBootCodeHandler(bootCodeService *service.BootCodeService, logger *slog.Logger) *BootCodeHandler {
	return &BootCodeHandler{bootCodeService: bootCodeService, logger: logger}
}

// Generate 会员基于已确认预约生成动态开机码。
func (h *BootCodeHandler) Generate(c *gin.Context) {
	var req dto.GenerateBootCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, constants.CodeValidation, "开机码生成参数校验失败："+err.Error())
		return
	}
	uid, _ := c.Get("user_id")
	userID, _ := uid.(uint)
	bc, err := h.bootCodeService.Generate(userID, &req)
	if err != nil {
		h.abort(c, err)
		return
	}
	response.OKMessage(c, constants.MsgBootCodeGenerateOK, toBootCodeResp(bc))
}

// Mine 会员查询自己当前有效的开机码。
func (h *BootCodeHandler) Mine(c *gin.Context) {
	var query struct {
		ReservationID uint `form:"reservation_id"`
	}
	_ = c.ShouldBindQuery(&query)
	uid, _ := c.Get("user_id")
	userID, _ := uid.(uint)
	bc, err := h.bootCodeService.Mine(userID, query.ReservationID)
	if err != nil {
		h.abort(c, err)
		return
	}
	if bc == nil {
		response.OK(c, nil)
		return
	}
	response.OK(c, toBootCodeResp(bc))
}

// Verify 店员扫码校验并开机，返回上机详情。
func (h *BootCodeHandler) Verify(c *gin.Context) {
	var req dto.VerifyBootCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, constants.CodeValidation, "扫码校验参数失败："+err.Error())
		return
	}
	uid, _ := c.Get("user_id")
	staffID, _ := uid.(uint)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	detail, err := h.bootCodeService.Verify(staffID, roleStr, &req)
	if err != nil {
		h.abort(c, err)
		return
	}
	response.OKMessage(c, constants.MsgBootCodeVerifyOK, detail)
}

// List 店员/管理员分页查询开机码核销记录。
func (h *BootCodeHandler) List(c *gin.Context) {
	var query dto.BootCodeQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Fail(c, 400, constants.CodeValidation, "开机码查询参数校验失败："+err.Error())
		return
	}
	list, total, err := h.bootCodeService.List(&query)
	if err != nil {
		h.abort(c, err)
		return
	}
	items := make([]dto.BootCodeResp, 0, len(list))
	for i := range list {
		items = append(items, toBootCodeResp(&list[i]))
	}
	response.OK(c, dto.PageResult{List: items, Total: total, Page: query.Page, PageSize: query.PageSize})
}

// toBootCodeResp 模型转响应 DTO。
func toBootCodeResp(bc *model.BootCode) dto.BootCodeResp {
	return dto.BootCodeResp{
		ID:            bc.ID,
		Code:          bc.Code,
		ReservationID: bc.ReservationID,
		UserID:        bc.UserID,
		StationID:     bc.StationID,
		SessionID:     bc.SessionID,
		Status:        bc.Status,
		ExpireAt:      bc.ExpireAt,
		VerifiedBy:    bc.VerifiedBy,
		VerifiedAt:    bc.VerifiedAt,
		CreatedAt:     bc.CreatedAt,
	}
}

// abort 统一错误处理（与其他 handler 保持一致：再次包装 service 错误）。
func (h *BootCodeHandler) abort(c *gin.Context, err error) {
	var appErr *util.AppError
	if errors.As(err, &appErr) {
		response.Fail(c, httpStatusFor(appErr.Code), appErr.Code, appErr.Message)
		return
	}
	h.logger.Error(fmt.Sprintf("boot code handler error: %v", err))
	response.Fail(c, 500, constants.CodeInternal, "服务器内部错误，请稍后重试")
}
