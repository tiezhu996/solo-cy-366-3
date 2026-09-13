package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/esportsbar/backend/internal/constants"
	"github.com/esportsbar/backend/internal/dto"
	"github.com/esportsbar/backend/internal/model"
	"github.com/esportsbar/backend/internal/repository"
	"github.com/esportsbar/backend/internal/util"
)

// SessionService 上机记录服务（含排行榜）。
type SessionService struct {
	sessionRepo     *repository.SessionRepository
	stationService  *StationService
	userPkgRepo     *repository.UserPackageRepository
	userRepo        *repository.UserRepository
	reservationRepo *repository.ReservationRepository
	db              *gorm.DB
	logger          *slog.Logger
}

// NewSessionService 构造上机记录服务。
func NewSessionService(
	sessionRepo *repository.SessionRepository,
	stationService *StationService,
	userPkgRepo *repository.UserPackageRepository,
	userRepo *repository.UserRepository,
	reservationRepo *repository.ReservationRepository,
	db *gorm.DB,
	logger *slog.Logger,
) *SessionService {
	return &SessionService{sessionRepo: sessionRepo, stationService: stationService, userPkgRepo: userPkgRepo, userRepo: userRepo, reservationRepo: reservationRepo, db: db, logger: logger}
}

// Start 会员上机开机：优先扣时长包，不足扣余额，事务内锁定机位。
func (s *SessionService) Start(userID uint, req *dto.StartSessionReq) (*model.Session, error) {
	station, err := s.stationService.GetByID(req.StationID)
	if err != nil {
		return nil, err
	}
	if station.Status != constants.StationIdle && station.Status != constants.StationReserved {
		return nil, util.NewAppError(constants.CodeStationBusy, "机位当前不可开机")
	}
	sess := &model.Session{
		UserID:        userID,
		StationID:     req.StationID,
		ReservationID: req.ReservationID,
		StartTime:     time.Now(),
		GameType:      defaultGameType(req.GameType),
		Status:        constants.SessionActive,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		locked, err := s.stationService.LockForUpdate(tx, req.StationID)
		if err != nil {
			return err
		}
		if locked.Status == constants.StationUsing {
			return util.NewAppError(constants.CodeSessionOpen, "该机位已有进行中的上机记录")
		}
		locked.Status = constants.StationUsing
		if err := tx.Save(locked).Error; err != nil {
			return err
		}
		if req.ReservationID > 0 {
			res, err := s.reservationRepo.FindByID(req.ReservationID)
			if err == nil && res.Status == constants.ReservationConfirmed {
				res.Status = constants.ReservationCheckedIn
				if err := tx.Save(res).Error; err != nil {
					return err
				}
			}
		}
		return s.sessionRepo.Create(sess)
	})
	if err != nil {
		return nil, fmt.Errorf("session start tx: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogTemplates["session_start_ok"], userID, req.StationID, sess.ID))
	return sess, nil
}

// Renew 续费：延长上机结束时间，并从时长包/余额扣除费用。
func (s *SessionService) Renew(userID, sessionID uint, req *dto.RenewSessionReq) (*model.Session, error) {
	sess, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, "上机记录不存在")
		}
		return nil, fmt.Errorf("session renew find: %w", err)
	}
	if sess.Status != constants.SessionActive {
		return nil, util.NewAppError(constants.CodeConflict, "仅进行中的上机可以续费")
	}
	station, err := s.stationService.GetByID(sess.StationID)
	if err != nil {
		return nil, err
	}
	// 先扣时长包，不足部分按机位时价扣余额。
	neededHours := float64(req.AddMinutes) / 60
	if err := s.consumeHoursAndCharge(userID, neededHours, station.PricePerHour); err != nil {
		return nil, err
	}
	now := time.Now()
	if sess.EndTime == nil {
		sess.EndTime = &now
	}
	end := sess.EndTime.Add(time.Duration(req.AddMinutes) * time.Minute)
	sess.EndTime = &end
	// 续费已即时扣费，分钟数计入预付，下机结算时不再重复收取。
	sess.PrepaidMinutes += req.AddMinutes
	if err := s.sessionRepo.Update(sess); err != nil {
		return nil, fmt.Errorf("session renew update: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogTemplates["session_renew_ok"], sessionID, req.AddMinutes))
	return sess, nil
}

// End 会员下机：结算时长与金额，释放机位。
func (s *SessionService) End(userID, sessionID uint, req *dto.EndSessionReq) (*model.Session, error) {
	sess, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, "上机记录不存在")
		}
		return nil, fmt.Errorf("session end find: %w", err)
	}
	if sess.Status != constants.SessionActive {
		return nil, util.NewAppError(constants.CodeConflict, "上机记录已结束")
	}
	now := time.Now()
	end := now
	if sess.EndTime != nil && sess.EndTime.After(now) {
		end = *sess.EndTime
	}
	duration := int(end.Sub(sess.StartTime).Minutes())
	// 扫码开机已按预约时长预付，下机只结算预付时段之外的超时部分（时长包优先、余额兜底）。
	billMinutes := duration - sess.PrepaidMinutes
	if billMinutes < 0 {
		billMinutes = 0
	}
	station, err := s.stationService.GetByID(sess.StationID)
	if err != nil {
		return nil, err
	}
	overtimeAmount := 0.0
	if billMinutes > 0 {
		neededHours := float64(billMinutes) / 60
		var charge ChargeResult
		charge, err = s.ChargeWithinTx(nil, userID, neededHours, station.PricePerHour)
		if err != nil {
			return nil, err
		}
		overtimeAmount = charge.BalanceCost
	}
	amount := station.PricePerHour*float64(sess.PrepaidMinutes)/60 + overtimeAmount
	err = s.db.Transaction(func(tx *gorm.DB) error {
		sess.EndTime = &end
		sess.DurationMinutes = duration
		sess.Amount = amount
		sess.GameType = defaultGameType(req.GameType)
		sess.Status = constants.SessionCompleted
		if err := s.sessionRepo.Update(sess); err != nil {
			return err
		}
		locked, err := s.stationService.LockForUpdate(tx, sess.StationID)
		if err != nil {
			return err
		}
		locked.Status = constants.StationIdle
		return tx.Save(locked).Error
	})
	if err != nil {
		return nil, fmt.Errorf("session end tx: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogTemplates["session_end_ok"], sessionID, duration, amount))
	return sess, nil
}

// consumeHoursAndCharge 消费扣款：优先消耗时长包小时数，不足部分按机位时价从余额扣除。
func (s *SessionService) consumeHoursAndCharge(userID uint, neededHours, pricePerHour float64) error {
	_, err := s.ChargeWithinTx(nil, userID, neededHours, pricePerHour)
	return err
}

// ChargeResult 计费拆账结果。
type ChargeResult struct {
	PackageHours float64 // 时长包承担小时数
	BalanceCost  float64 // 余额承担金额
}

// ChargeWithinTx 在指定事务内完成"时长包优先、余额兜底"的计费扣费，返回拆账结果。
// tx 为 nil 时使用仓储自带事务；扫码开机（boot_service）、续费、下机超时结算均复用该方法。
func (s *SessionService) ChargeWithinTx(tx *gorm.DB, userID uint, neededHours, pricePerHour float64) (ChargeResult, error) {
	if neededHours <= 0 {
		return ChargeResult{}, nil
	}
	var (
		consumedHours float64
		err           error
	)
	if tx != nil {
		consumedHours, err = s.userPkgRepo.ConsumeHoursTx(tx, userID, neededHours)
	} else {
		consumedHours, err = s.userPkgRepo.ConsumeHours(userID, neededHours)
	}
	if err != nil {
		return ChargeResult{}, fmt.Errorf("session consume package: %w", err)
	}
	packageHours, balanceCost := splitBilling(neededHours, consumedHours, pricePerHour)
	if balanceCost > 0 {
		if tx != nil {
			err = s.userRepo.UpdateBalanceTx(tx, userID, -balanceCost)
		} else {
			err = s.userRepo.UpdateBalance(userID, -balanceCost)
		}
		if err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return ChargeResult{}, util.NewAppError(constants.CodeInsufficient, "会员余额不足，请先充值")
			}
			return ChargeResult{}, fmt.Errorf("session consume balance: %w", err)
		}
	}
	return ChargeResult{PackageHours: packageHours, BalanceCost: balanceCost}, nil
}

// List 分页查询上机记录。
func (s *SessionService) List(page, pageSize int, userID uint, status string) ([]model.Session, int64, error) {
	return s.sessionRepo.List(page, pageSize, userID, status)
}

// Rank 上机时长排行榜。
func (s *SessionService) Rank(query *dto.RankQuery) ([]RankItem, error) {
	period := query.Period
	if period == "" {
		period = "week"
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}
	rows, _, err := s.sessionRepo.Rank(period, query.GameType, limit)
	if err != nil {
		return nil, fmt.Errorf("session rank: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogTemplates["session_rank_query"], period, query.GameType))
	items := make([]RankItem, 0, len(rows))
	for i, row := range rows {
		user, err := s.userRepo.FindByID(row.UserID)
		if err != nil {
			continue
		}
		items = append(items, RankItem{
			Rank:         i + 1,
			UserID:       row.UserID,
			Username:     user.Username,
			Nickname:     user.Nickname,
			TotalMinutes: row.DurationMinutes,
		})
	}
	return items, nil
}

// RankItem 排行榜条目。
type RankItem struct {
	Rank         int    `json:"rank"`
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	Nickname     string `json:"nickname"`
	TotalMinutes int    `json:"total_minutes"`
}

// defaultGameType 默认游戏类型。
func defaultGameType(gameType string) string {
	if gameType == "" {
		return constants.GameOther
	}
	return gameType
}
