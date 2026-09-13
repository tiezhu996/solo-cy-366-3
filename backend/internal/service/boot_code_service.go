package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"gorm.io/gorm"

	"github.com/esportsbar/backend/internal/constants"
	"github.com/esportsbar/backend/internal/dto"
	"github.com/esportsbar/backend/internal/model"
	"github.com/esportsbar/backend/internal/repository"
	"github.com/esportsbar/backend/internal/util"
)

// 到店时间窗：允许在预约开始前 BootArriveEarlyMinutes 分钟内生成开机码。
const BootArriveEarlyMinutes = 15

// BootCodeService 会员动态开机码与店员扫码开机服务。
type BootCodeService struct {
	bootCodeRepo    *repository.BootCodeRepository
	reservationRepo *repository.ReservationRepository
	sessionRepo     *repository.SessionRepository
	userRepo        *repository.UserRepository
	userPkgRepo     *repository.UserPackageRepository
	stationService  *StationService
	sessionService  *SessionService
	db              *gorm.DB
	logger          *slog.Logger
}

// NewBootCodeService 构造动态开机码服务。
func NewBootCodeService(
	bootCodeRepo *repository.BootCodeRepository,
	reservationRepo *repository.ReservationRepository,
	sessionRepo *repository.SessionRepository,
	userRepo *repository.UserRepository,
	userPkgRepo *repository.UserPackageRepository,
	stationService *StationService,
	sessionService *SessionService,
	db *gorm.DB,
	logger *slog.Logger,
) *BootCodeService {
	return &BootCodeService{
		bootCodeRepo:    bootCodeRepo,
		reservationRepo: reservationRepo,
		sessionRepo:     sessionRepo,
		userRepo:        userRepo,
		userPkgRepo:     userPkgRepo,
		stationService:  stationService,
		sessionService:  sessionService,
		db:              db,
		logger:          logger,
	}
}

// Generate 会员到店后基于已确认预约生成动态开机码（同一预约旧码自动作废）。
func (s *BootCodeService) Generate(memberID uint, req *dto.GenerateBootCodeReq) (*model.BootCode, error) {
	res, err := s.reservationRepo.FindByID(req.ReservationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, fmt.Sprintf("预约(reservation_id=%d)不存在，会员(user_id=%d)无法生成开机码", req.ReservationID, memberID))
		}
		return nil, fmt.Errorf("boot code generate find reservation: %w", err)
	}
	if res.UserID != memberID {
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf("预约(reservation_id=%d)不属于当前会员(user_id=%d)，角色%s无权生成开机码", res.ID, memberID, constants.RoleMember))
	}
	if res.Status != constants.ReservationConfirmed {
		return nil, util.NewAppError(constants.CodeReservation, fmt.Sprintf("预约(reservation_id=%d)当前状态为%s，仅%s状态的预约可生成开机码", res.ID, util.StatusText(res.Status), util.StatusText(constants.ReservationConfirmed)))
	}
	now := time.Now()
	if beforeWindow, expired := inArrivalWindow(now, res.StartTime, res.EndTime); beforeWindow || expired {
		if expired {
			return nil, util.NewAppError(constants.CodeBootWindow, fmt.Sprintf("预约(reservation_id=%d)已于 %s 过期，无法生成开机码", res.ID, res.EndTime.Format("2006-01-02 15:04")))
		}
		return nil, util.NewAppError(constants.CodeBootWindow, fmt.Sprintf("未到到店时间窗：预约 %s 开始，提前 %d 分钟内才可生成开机码", res.StartTime.Format("2006-01-02 15:04"), BootArriveEarlyMinutes))
	}
	// 生成前再校验一次机位未被他人占用（故障机位同样不允许）。
	station, err := s.stationService.GetByID(res.StationID)
	if err != nil {
		return nil, err
	}
	if station.Status == constants.StationUsing || station.Status == constants.StationFault {
		return nil, util.NewAppError(constants.CodeStationBusy, fmt.Sprintf("机位(station_id=%d,%s)当前状态为%s，暂不可开机", station.ID, station.Name, util.StatusText(station.Status)))
	}

	var boot *model.BootCode
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 锁定预约，防止会员并发重复生成。
		lockedRes, err := s.reservationRepo.FindByIDForUpdate(tx, res.ID)
		if err != nil {
			return err
		}
		if lockedRes.Status != constants.ReservationConfirmed {
			return util.NewAppError(constants.CodeReservation, "预约状态已变更，请刷新后重试")
		}
		if err := s.bootCodeRepo.RevokeActiveByReservation(tx, res.ID, constants.BootCodeCancelled, now); err != nil {
			return fmt.Errorf("revoke old boot codes: %w", err)
		}
		code, err := s.uniqueCodeTx(tx)
		if err != nil {
			return err
		}
		boot = &model.BootCode{
			Code:          code,
			ReservationID: res.ID,
			UserID:        res.UserID,
			StationID:     res.StationID,
			Status:        constants.BootCodeActive,
			ExpireAt:      now.Add(constants.BootCodeTTLMinutes * time.Minute),
		}
		if err := s.bootCodeRepo.CreateTx(tx, boot); err != nil {
			return fmt.Errorf("create boot code: %w", err)
		}
		return nil
	})
	if err != nil {
		var appErr *util.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, fmt.Errorf("boot code generate tx: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogTemplates["boot_code_generate_ok"], memberID, res.ID, res.StationID, boot.Code))
	return boot, nil
}

// Mine 查询会员自己当前有效（待核销且未过期）的开机码，没有则返回 nil。
func (s *BootCodeService) Mine(memberID uint, reservationID uint) (*model.BootCode, error) {
	var (
		bc  *model.BootCode
		err error
	)
	if reservationID > 0 {
		bc, err = s.bootCodeRepo.FindActiveByReservation(s.db, reservationID, time.Now())
	} else {
		bc, err = s.bootCodeRepo.FindLatestActiveByUser(memberID, time.Now())
	}
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("boot code mine: %w", err)
	}
	if bc.UserID != memberID {
		return nil, util.NewAppError(constants.CodeForbidden, "无权查看该开机码")
	}
	return bc, nil
}

// Verify 店员扫码校验并开机：校验动态码、预约、机位、有效期、角色，
// 事务内占用机位、创建上机记录、按预约时长扣减时长包/余额、核销开机码。
func (s *BootCodeService) Verify(staffID uint, staffRole string, req *dto.VerifyBootCodeReq) (*dto.BootSessionDetail, error) {
	if staffRole != constants.RoleStaff && staffRole != constants.RoleAdmin {
		return nil, util.NewAppError(constants.CodeForbidden, fmt.Sprintf("扫码开机仅允许%s/%s，当前角色为%s", constants.RoleStaff, constants.RoleAdmin, staffRole))
	}

	var (
		detail  *dto.BootSessionDetail
		failMsg string
	)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 行锁动态码，防止并发重复核销。
		bc, err := s.bootCodeRepo.FindByCodeForUpdate(tx, req.Code)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				failMsg = "动态开机码无效或不存在"
				return util.NewAppError(constants.CodeBootNotFound, failMsg)
			}
			return fmt.Errorf("find boot code: %w", err)
		}
		switch code := checkBootCodeStatus(bc.Status, time.Now(), bc.ExpireAt); code {
		case constants.CodeBootUsed:
			failMsg = fmt.Sprintf("开机码 %s 已核销", bc.Code)
			return util.NewAppError(code, failMsg)
		case constants.CodeBootExpired:
			if bc.Status == constants.BootCodeCancelled {
				failMsg = fmt.Sprintf("开机码 %s 已作废（会员可能已重新生成）", bc.Code)
			} else if bc.Status == constants.BootCodeActive {
				failMsg = fmt.Sprintf("开机码 %s 已超过有效期（%s），请让会员重新生成", bc.Code, bc.ExpireAt.Format("15:04:05"))
			} else {
				failMsg = fmt.Sprintf("开机码 %s 已超过有效期", bc.Code)
			}
			return util.NewAppError(code, failMsg)
		case constants.CodeBootNotFound:
			failMsg = bootInvalidMessage(bc.Code, bc.Status)
			return util.NewAppError(code, failMsg)
		}

		// 2. 行锁预约，校验归属与状态。
		res, err := s.reservationRepo.FindByIDForUpdate(tx, bc.ReservationID)
		if err != nil {
			return fmt.Errorf("find reservation: %w", err)
		}
		if res.UserID != bc.UserID || res.StationID != bc.StationID {
			failMsg = fmt.Sprintf("开机码与预约(reservation_id=%d)信息不匹配", bc.ReservationID)
			return util.NewAppError(constants.CodeBootMismatch, failMsg)
		}
		if res.Status == constants.ReservationCancelled {
			failMsg = fmt.Sprintf("预约(reservation_id=%d)已取消，开机码无效", res.ID)
			return util.NewAppError(constants.CodeBootMismatch, failMsg)
		}
		if res.Status == constants.ReservationCheckedIn {
			failMsg = fmt.Sprintf("预约(reservation_id=%d)已开机，请勿重复扫码", res.ID)
			return util.NewAppError(constants.CodeBootUsed, failMsg)
		}
		if res.Status != constants.ReservationConfirmed {
			failMsg = fmt.Sprintf("预约(reservation_id=%d)状态为%s，不可开机", res.ID, util.StatusText(res.Status))
			return util.NewAppError(constants.CodeReservation, failMsg)
		}
		if time.Now().After(res.EndTime) {
			failMsg = fmt.Sprintf("预约(reservation_id=%d)已于 %s 过期", res.ID, res.EndTime.Format("2006-01-02 15:04"))
			return util.NewAppError(constants.CodeBootWindow, failMsg)
		}

		// 3. 行锁机位，校验空闲状态；店员可携带机位码二次核对。
		station, err := s.stationService.LockForUpdate(tx, bc.StationID)
		if err != nil {
			return fmt.Errorf("lock station: %w", err)
		}
		if req.StationID > 0 && req.StationID != station.ID {
			failMsg = fmt.Sprintf("扫码机位(station_id=%d)与预约机位(station_id=%d)不一致", req.StationID, station.ID)
			return util.NewAppError(constants.CodeBootMismatch, failMsg)
		}
		if station.Status == constants.StationUsing {
			failMsg = fmt.Sprintf("机位(%s)正在使用中，无法开机", station.Name)
			return util.NewAppError(constants.CodeStationBusy, failMsg)
		}
		if station.Status == constants.StationFault {
			failMsg = fmt.Sprintf("机位(%s)处于故障状态，无法开机", station.Name)
			return util.NewAppError(constants.CodeStationFault, failMsg)
		}
		if active, err := s.sessionRepo.FindActiveByStation(station.ID); err == nil && active != nil {
			failMsg = fmt.Sprintf("机位(%s)已有进行中的上机记录(session_id=%d)", station.Name, active.ID)
			return util.NewAppError(constants.CodeSessionOpen, failMsg)
		}

		// 4. 行锁会员，校验账户状态。
		member, err := s.userRepo.FindByIDForUpdate(tx, bc.UserID)
		if err != nil {
			return fmt.Errorf("find member: %w", err)
		}
		if member.Role != constants.RoleMember {
			failMsg = fmt.Sprintf("开机码对应用户(%s)角色为%s，不是会员", member.Username, util.RoleText(member.Role))
			return util.NewAppError(constants.CodeForbidden, failMsg)
		}
		if member.Status != "active" {
			failMsg = fmt.Sprintf("会员(%s)账户已停用，无法开机", member.Username)
			return util.NewAppError(constants.CodeForbidden, failMsg)
		}

		// 5. 按预约时长计费：时长包优先，不足余额兜底（与续费/下机复用 SessionService.ChargeWithinTx）。
		startAt := time.Now()
		endAt := res.EndTime
		if endAt.Before(startAt) {
			endAt = startAt
		}
		prepaidMinutes := int(endAt.Sub(startAt).Minutes())
		if prepaidMinutes < 0 {
			prepaidMinutes = 0
		}
		neededHours := float64(prepaidMinutes) / 60
		balanceBefore := member.Balance
		charge, err := s.sessionService.ChargeWithinTx(tx, member.ID, neededHours, station.PricePerHour)
		if err != nil {
			var appErr *util.AppError
			if errors.As(err, &appErr) {
				failMsg = appErr.Message
				return appErr
			}
			return fmt.Errorf("boot charge: %w", err)
		}
		remainingHours, err := s.userPkgRepo.SumRemainingHours(tx, member.ID)
		if err != nil {
			return fmt.Errorf("sum remaining hours: %w", err)
		}

		// 6. 状态落库：占用机位、预约 checked_in、创建上机记录、核销开机码。
		station.Status = constants.StationUsing
		if err := tx.Save(station).Error; err != nil {
			return fmt.Errorf("save station: %w", err)
		}
		res.Status = constants.ReservationCheckedIn
		if err := tx.Save(res).Error; err != nil {
			return fmt.Errorf("save reservation: %w", err)
		}
		verifiedAt := time.Now()
		sess := &model.Session{
			UserID:         member.ID,
			StationID:      station.ID,
			ReservationID:  res.ID,
			StartTime:      startAt,
			EndTime:        &endAt,
			PrepaidMinutes: prepaidMinutes,
			Amount:         station.PricePerHour * neededHours,
			GameType:       constants.GameOther,
			Status:         constants.SessionActive,
		}
		if err := s.sessionRepo.CreateTx(tx, sess); err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		bc.Status = constants.BootCodeUsed
		bc.SessionID = sess.ID
		bc.VerifiedBy = staffID
		bc.VerifiedAt = &verifiedAt
		if err := tx.Save(bc).Error; err != nil {
			return fmt.Errorf("save boot code: %w", err)
		}

		detail = &dto.BootSessionDetail{
			SessionID:       sess.ID,
			Code:            bc.Code,
			ReservationID:   res.ID,
			UserID:          member.ID,
			Username:        member.Username,
			Nickname:        member.Nickname,
			Phone:           member.Phone,
			StationID:       station.ID,
			StationName:     station.Name,
			Area:            station.Area,
			StationType:     station.StationType,
			StartTime:       sess.StartTime,
			ReservedEndTime: endAt,
			PricePerHour:    station.PricePerHour,
			PackageHours:    round2(charge.PackageHours),
			BalanceCost:     round2(charge.BalanceCost),
			RemainingHours:  round2(remainingHours),
			Balance:         round2(balanceBefore - charge.BalanceCost),
			Status:          sess.Status,
			ExpireAt:        bc.ExpireAt,
			VerifiedAt:      bc.VerifiedAt,
		}
		s.logger.Info(fmt.Sprintf(constants.LogTemplates["boot_code_consume_ok"], member.ID, charge.PackageHours, charge.BalanceCost))
		return nil
	})
	if err != nil {
		var appErr *util.AppError
		if errors.As(err, &appErr) {
			// 动态码超过有效期时，事务已回滚；在事务外幂等落库 expired 状态。
			if appErr.Code == constants.CodeBootExpired {
				if markErr := s.bootCodeRepo.MarkExpiredByCode(req.Code, time.Now()); markErr != nil {
					s.logger.Warn(fmt.Sprintf("mark boot code expired failed, code=%s, err=%v", req.Code, markErr))
				}
			}
			s.logger.Warn(fmt.Sprintf(constants.LogTemplates["boot_code_verify_fail"], req.Code, staffID, failMsg))
			return nil, appErr
		}
		return nil, fmt.Errorf("boot code verify tx: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogTemplates["boot_code_verify_ok"], req.Code, staffID, detail.UserID, detail.StationID, detail.SessionID))
	return detail, nil
}

// List 分页查询开机码核销记录（店员/管理员）。
func (s *BootCodeService) List(query *dto.BootCodeQuery) ([]model.BootCode, int64, error) {
	page := query.Page
	if page <= 0 {
		page = constants.DefaultPage
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = constants.DefaultPageSize
	}
	return s.bootCodeRepo.List(page, pageSize, query.Status, query.ReservationID, query.UserID)
}

// uniqueCodeTx 在事务内生成不冲突的 6 位数字动态码。
func (s *BootCodeService) uniqueCodeTx(tx *gorm.DB) (string, error) {
	for i := 0; i < 5; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err != nil {
			return "", fmt.Errorf("gen random code: %w", err)
		}
		code := fmt.Sprintf("%06d", n.Int64()+100000)
		exists, err := s.bootCodeRepo.CodeExistsTx(tx, code)
		if err != nil {
			return "", fmt.Errorf("check code exists: %w", err)
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("generate unique boot code exhausted")
}

// round2 保留两位小数，避免金额展示浮点误差。
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// checkBootCodeStatus 校验开机码状态机：返回错误码，0 表示可继续核销。
// 状态流转 active → used（核销）/ expired（超时）/ cancelled（重新生成作废旧码）。
func checkBootCodeStatus(status string, now, expireAt time.Time) int {
	switch status {
	case constants.BootCodeUsed:
		return constants.CodeBootUsed
	case constants.BootCodeExpired:
		return constants.CodeBootExpired
	case constants.BootCodeCancelled:
		return constants.CodeBootExpired
	case constants.BootCodeActive:
		if !now.Before(expireAt) {
			return constants.CodeBootExpired
		}
		return constants.CodeOK
	}
	return constants.CodeBootNotFound
}

// bootInvalidMessage 拼装非法状态开机码的失败提示（含实体名、字段名、状态）。
func bootInvalidMessage(code, status string) string {
	return fmt.Sprintf("开机码(code=%s)状态字段 status=%s 非法，无法核销", code, status)
}

// inArrivalWindow 判断当前时间是否在预约到店时间窗内（开始前 BootArriveEarlyMinutes 分钟 ~ 预约结束）。
// 返回 beforeWindow/expired 均为 false 表示在窗内。
func inArrivalWindow(now, start, end time.Time) (beforeWindow bool, expired bool) {
	return now.Before(start.Add(-BootArriveEarlyMinutes * time.Minute)), now.After(end)
}

// splitBilling 将所需小时拆成时长包承担与余额承担两部分（时长包优先），返回复用扣包后的余额计费金额。
func splitBilling(neededHours, consumedHours, pricePerHour float64) (packageHours, balanceCost float64) {
	packageHours = consumedHours
	if packageHours > neededHours {
		packageHours = neededHours
	}
	remaining := neededHours - packageHours
	if remaining > 0.0001 {
		balanceCost = remaining * pricePerHour
	}
	return packageHours, balanceCost
}
