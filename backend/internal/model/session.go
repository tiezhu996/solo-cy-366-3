package model

import "time"

// Session 上机记录。
type Session struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	UserID          uint       `gorm:"index;not null" json:"user_id"`
	StationID       uint       `gorm:"index;not null" json:"station_id"`
	ReservationID   uint       `gorm:"index" json:"reservation_id"`
	StartTime       time.Time  `gorm:"not null" json:"start_time"`
	EndTime         *time.Time `json:"end_time"`
	DurationMinutes int        `gorm:"default:0" json:"duration_minutes"`
	PrepaidMinutes  int        `gorm:"default:0" json:"prepaid_minutes"` // 扫码开机时按预约时长预付的分钟数（下机不重复计费）
	GameType        string     `gorm:"size:16;default:other" json:"game_type"`
	Amount          float64    `gorm:"type:decimal(12,2);default:0" json:"amount"`
	Status          string     `gorm:"size:16;default:active" json:"status"` // active/completed
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (Session) TableName() string { return "sessions" }
