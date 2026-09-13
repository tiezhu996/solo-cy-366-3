package model

import "time"

// BootCode 会员到店动态开机码。会员在已确认预约到达后生成，店员扫码核销后完成开机。
type BootCode struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Code          string     `gorm:"size:16;uniqueIndex;not null" json:"code"` // 6 位动态数字码
	ReservationID uint       `gorm:"index;not null" json:"reservation_id"`
	UserID        uint       `gorm:"index;not null" json:"user_id"`
	StationID     uint       `gorm:"index;not null" json:"station_id"`
	SessionID     uint       `gorm:"index;default:0" json:"session_id"`    // 核销开机后回填上机记录
	Status        string     `gorm:"size:16;default:active" json:"status"` // active/used/expired/cancelled
	ExpireAt      time.Time  `gorm:"not null" json:"expire_at"`
	VerifiedBy    uint       `gorm:"index;default:0" json:"verified_by"` // 核销店员 ID
	VerifiedAt    *time.Time `json:"verified_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (BootCode) TableName() string { return "boot_codes" }
