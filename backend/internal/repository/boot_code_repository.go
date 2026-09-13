package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/esportsbar/backend/internal/model"
)

// BootCodeRepository 动态开机码仓储。
type BootCodeRepository struct {
	db *gorm.DB
}

// NewBootCodeRepository 构造开机码仓储。
func NewBootCodeRepository(db *gorm.DB) *BootCodeRepository {
	return &BootCodeRepository{db: db}
}

// Create 创建开机码。
func (r *BootCodeRepository) Create(bc *model.BootCode) error {
	return r.db.Create(bc).Error
}

// CreateTx 在指定事务内创建开机码。
func (r *BootCodeRepository) CreateTx(tx *gorm.DB, bc *model.BootCode) error {
	return tx.Create(bc).Error
}

// CodeExistsTx 事务内判断动态码是否已存在（生成时碰撞重试）。
func (r *BootCodeRepository) CodeExistsTx(tx *gorm.DB, code string) (bool, error) {
	var cnt int64
	if err := tx.Model(&model.BootCode{}).Where("code = ?", code).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// FindByCode 按动态码查询。
func (r *BootCodeRepository) FindByCode(code string) (*model.BootCode, error) {
	var bc model.BootCode
	err := r.db.Where("code = ?", code).First(&bc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &bc, err
}

// FindByCodeForUpdate 事务内行锁查询动态码，防止并发重复核销。
func (r *BootCodeRepository) FindByCodeForUpdate(tx *gorm.DB, code string) (*model.BootCode, error) {
	var bc model.BootCode
	err := tx.Clauses(clauseLocking()).Where("code = ?", code).First(&bc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &bc, err
}

// FindByID 按 ID 查询。
func (r *BootCodeRepository) FindByID(id uint) (*model.BootCode, error) {
	var bc model.BootCode
	err := r.db.First(&bc, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &bc, err
}

// Update 更新开机码。
func (r *BootCodeRepository) Update(bc *model.BootCode) error {
	return r.db.Save(bc).Error
}

// List 分页查询开机码（店员/管理员核销记录）。
func (r *BootCodeRepository) List(page, pageSize int, status string, reservationID, userID uint) ([]model.BootCode, int64, error) {
	var list []model.BootCode
	var total int64
	query := r.db.Model(&model.BootCode{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if reservationID > 0 {
		query = query.Where("reservation_id = ?", reservationID)
	}
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// FindActiveByReservation 查询某预约当前有效（待核销且未过期）的开机码。
func (r *BootCodeRepository) FindActiveByReservation(tx *gorm.DB, reservationID uint, now time.Time) (*model.BootCode, error) {
	var bc model.BootCode
	err := tx.Clauses(clauseLocking()).
		Where("reservation_id = ? AND status = ? AND expire_at > ?", reservationID, "active", now).
		Order("id DESC").First(&bc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &bc, err
}

// FindLatestActiveByUser 查询会员最近一张有效（待核销且未过期）的开机码。
func (r *BootCodeRepository) FindLatestActiveByUser(userID uint, now time.Time) (*model.BootCode, error) {
	var bc model.BootCode
	err := r.db.
		Where("user_id = ? AND status = ? AND expire_at > ?", userID, "active", now).
		Order("id DESC").First(&bc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &bc, err
}

// RevokeActiveByReservation 作废旧码：将该预约下待核销的开机码置为指定状态。
func (r *BootCodeRepository) RevokeActiveByReservation(tx *gorm.DB, reservationID uint, status string, now time.Time) error {
	return tx.Model(&model.BootCode{}).
		Where("reservation_id = ? AND status = ?", reservationID, "active").
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

// MarkExpiredByCode 将已超过有效期但仍为 active 的动态码幂等置为 expired（开机事务回滚后调用）。
func (r *BootCodeRepository) MarkExpiredByCode(code string, now time.Time) error {
	return r.db.Model(&model.BootCode{}).
		Where("code = ? AND status = ? AND expire_at <= ?", code, "active", now).
		Updates(map[string]any{"status": "expired", "updated_at": now}).Error
}
