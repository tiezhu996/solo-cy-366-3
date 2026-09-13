package dto

import "time"

// GenerateBootCodeReq 会员生成动态开机码请求。
type GenerateBootCodeReq struct {
	ReservationID uint `json:"reservation_id" binding:"required"`
}

// VerifyBootCodeReq 店员扫码核销开机请求。
type VerifyBootCodeReq struct {
	Code      string `json:"code" binding:"required,min=6,max=6,numeric"`
	StationID uint   `json:"station_id" binding:"omitempty,min=1"` // 扫码时可选核对机位
}

// BootCodeQuery 开机码分页查询参数。
type BootCodeQuery struct {
	Page          int    `form:"page" binding:"omitempty,min=1"`
	PageSize      int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Status        string `form:"status" binding:"omitempty,oneof=active used expired cancelled"`
	ReservationID uint   `form:"reservation_id"`
	UserID        uint   `form:"user_id"`
}

// BootCodeResp 开机码响应（会员展示二维码、店员核销记录复用）。
type BootCodeResp struct {
	ID            uint       `json:"id"`
	Code          string     `json:"code"`
	ReservationID uint       `json:"reservation_id"`
	UserID        uint       `json:"user_id"`
	StationID     uint       `json:"station_id"`
	SessionID     uint       `json:"session_id"`
	Status        string     `json:"status"`
	ExpireAt      time.Time  `json:"expire_at"`
	VerifiedBy    uint       `json:"verified_by"`
	VerifiedAt    *time.Time `json:"verified_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// BootSessionDetail 扫码开机成功后返回的上机详情。
type BootSessionDetail struct {
	SessionID       uint       `json:"session_id"`
	Code            string     `json:"code"`
	ReservationID   uint       `json:"reservation_id"`
	UserID          uint       `json:"user_id"`
	Username        string     `json:"username"`
	Nickname        string     `json:"nickname"`
	Phone           string     `json:"phone"`
	StationID       uint       `json:"station_id"`
	StationName     string     `json:"station_name"`
	Area            string     `json:"area"`
	StationType     string     `json:"station_type"`
	StartTime       time.Time  `json:"start_time"`
	ReservedEndTime time.Time  `json:"reserved_end_time"`
	PricePerHour    float64    `json:"price_per_hour"`
	PackageHours    float64    `json:"package_hours"`   // 本次从时长包扣减的小时数
	BalanceCost     float64    `json:"balance_cost"`    // 本次从余额扣减的金额
	RemainingHours  float64    `json:"remaining_hours"` // 会员剩余时长包小时合计
	Balance         float64    `json:"balance"`         // 会员剩余余额
	Status          string     `json:"status"`
	ExpireAt        time.Time  `json:"expire_at"`
	VerifiedAt      *time.Time `json:"verified_at"`
}
