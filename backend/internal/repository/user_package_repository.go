package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/esportsbar/backend/internal/model"
)

// UserPackageRepository 会员时长包仓储。
type UserPackageRepository struct {
	db *gorm.DB
}

// NewUserPackageRepository 构造会员时长包仓储。
func NewUserPackageRepository(db *gorm.DB) *UserPackageRepository {
	return &UserPackageRepository{db: db}
}

// Create 创建会员时长包。
func (r *UserPackageRepository) Create(up *model.UserPackage) error {
	return r.db.Create(up).Error
}

// FindByID 查询会员时长包。
func (r *UserPackageRepository) FindByID(id uint) (*model.UserPackage, error) {
	var up model.UserPackage
	err := r.db.First(&up, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &up, err
}

// FindActiveByUser 查询会员有效时长包列表。
func (r *UserPackageRepository) FindActiveByUser(userID uint) ([]model.UserPackage, error) {
	var list []model.UserPackage
	err := r.db.Where("user_id = ? AND status = ?", userID, "active").
		Order("expire_at ASC").Find(&list).Error
	return list, err
}

// ConsumeHours 扣减时长包小时数（先扣最接近过期的）。
func (r *UserPackageRepository) ConsumeHours(userID uint, hours float64) (float64, error) {
	consumed := 0.0
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		consumed, err = r.ConsumeHoursTx(tx, userID, hours)
		return err
	})
	return consumed, err
}

// ConsumeHoursTx 在指定事务内扣减时长包，返回实际消耗小时数；扫码开机与续费/下机复用同一套扣包规则。
func (r *UserPackageRepository) ConsumeHoursTx(tx *gorm.DB, userID uint, hours float64) (float64, error) {
	consumed := 0.0
	var list []model.UserPackage
	if err := tx.Clauses(clauseLocking()).
		Where("user_id = ? AND status = ? AND remaining_hours > 0", userID, "active").
		Order("expire_at ASC").Find(&list).Error; err != nil {
		return 0, err
	}
	need := hours
	for i := range list {
		if need <= 0 {
			break
		}
		up := &list[i]
		if up.ExpireAt != nil && up.ExpireAt.Before(time.Now()) {
			up.Status = "expired"
			if err := tx.Save(up).Error; err != nil {
				return 0, err
			}
			continue
		}
		use := need
		if up.RemainingHours < use {
			use = up.RemainingHours
		}
		up.RemainingHours -= use
		need -= use
		consumed += use
		if up.RemainingHours <= 0.0001 {
			up.Status = "used"
		}
		if err := tx.Save(up).Error; err != nil {
			return 0, err
		}
	}
	return consumed, nil
}

// SumRemainingHours 统计会员时长包剩余小时合计。
func (r *UserPackageRepository) SumRemainingHours(tx *gorm.DB, userID uint) (float64, error) {
	var total float64
	err := tx.Model(&model.UserPackage{}).
		Where("user_id = ? AND status = ? AND remaining_hours > 0", userID, "active").
		Select("COALESCE(SUM(remaining_hours),0)").Scan(&total).Error
	return total, err
}

// CreditHours 充值/购买后为会员时长包增加小时数。
func (r *UserPackageRepository) CreditHours(tx *gorm.DB, up *model.UserPackage) error {
	return tx.Create(up).Error
}
