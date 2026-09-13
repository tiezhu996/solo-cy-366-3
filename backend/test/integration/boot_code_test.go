package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/esportsbar/backend/internal/constants"
	"github.com/esportsbar/backend/internal/model"
)

// TestVerifyValidCode_BootSuccess 有效开机码核销：占用机位、预约 checked_in、创建会话、扣费、码核销。
func TestVerifyValidCode_BootSuccess(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("A区-01", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	code, _ := env.generateCode(memberToken, res.ID)
	v := env.assertVerify(staffToken, code, 0, http.StatusOK, constants.CodeOK)

	// 接口返回上机详情
	if v.SessionID == 0 || v.StationID != st.ID || v.UserID != u.Member.ID ||
		v.Code != code || v.ReservationID != res.ID || v.Status != constants.SessionActive {
		t.Fatalf("[POST /boot-codes/verify] 上机详情字段异常: %+v（接口返回结构问题）", v)
	}
	if !(v.BalanceCost > 0) || !(v.Balance < 100) || v.PackageHours != 0 {
		t.Fatalf("[POST /boot-codes/verify] 纯余额扣费拆账异常: packageHours=%f balanceCost=%f balance=%f（计费逻辑问题）",
			v.PackageHours, v.BalanceCost, v.Balance)
	}

	// 数据库落库状态
	var station model.Station
	if err := env.db.First(&station, st.ID).Error; err != nil || station.Status != constants.StationUsing {
		t.Fatalf("[DB stations/%d] 核销后机位应为 using，got %q err=%v（事务状态流转问题）", st.ID, station.Status, err)
	}
	var reservation model.Reservation
	if err := env.db.First(&reservation, res.ID).Error; err != nil || reservation.Status != constants.ReservationCheckedIn {
		t.Fatalf("[DB reservations/%d] 核销后预约应为 checked_in，got %q（事务状态流转问题）", res.ID, reservation.Status)
	}
	var bc model.BootCode
	if err := env.db.Where("code = ?", code).First(&bc).Error; err != nil {
		t.Fatalf("[DB boot_codes code=%s] 查询失败: %v", code, err)
	}
	if bc.Status != constants.BootCodeUsed || bc.SessionID != v.SessionID || bc.VerifiedBy != u.Staff.ID || bc.VerifiedAt == nil {
		t.Fatalf("[DB boot_codes code=%s] 核销落库异常: status=%s sessionID=%d verifiedBy=%d verifiedAt=%v（核销记录问题）",
			code, bc.Status, bc.SessionID, bc.VerifiedBy, bc.VerifiedAt)
	}
	var sessions []model.Session
	if err := env.db.Where("reservation_id = ? AND status = ?", res.ID, constants.SessionActive).Find(&sessions).Error; err != nil || len(sessions) != 1 {
		t.Fatalf("[DB sessions] 应恰好 1 条 active 会话，got %d err=%v（上机记录留存问题）", len(sessions), err)
	}
	if sessions[0].EndTime == nil || sessions[0].PrepaidMinutes <= 0 {
		t.Fatalf("[DB sessions/%d] 预付结束时间/预付分钟数异常: end=%v prepaid=%d", sessions[0].ID, sessions[0].EndTime, sessions[0].PrepaidMinutes)
	}
}

// TestVerifyDuplicateCode_Rejected 同一开机码重复扫码必须拒绝。
func TestVerifyDuplicateCode_Rejected(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("A区-02", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	code, _ := env.generateCode(memberToken, res.ID)
	env.assertVerify(staffToken, code, 0, http.StatusOK, constants.CodeOK)

	// 第二次扫码：已核销码
	env.assertVerify(staffToken, code, 0, http.StatusConflict, constants.CodeBootUsed)

	var cnt int64
	env.db.Model(&model.Session{}).Where("reservation_id = ?", res.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("[DB sessions] 重复扫码不应再建会话，got %d 条（重复核销防护问题）", cnt)
	}
}

// TestVerifyExpiredCode_Rejected 超过有效期的动态码核销被拒，且状态落库 expired。
func TestVerifyExpiredCode_Rejected(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("A区-03", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	code, _ := env.generateCode(memberToken, res.ID)
	// 模拟码已过有效期
	if err := env.db.Model(&model.BootCode{}).Where("code = ?", code).
		Update("expire_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("update expire_at: %v", err)
	}
	env.assertVerify(staffToken, code, 0, http.StatusConflict, constants.CodeBootExpired)

	var bc model.BootCode
	env.db.Where("code = ?", code).First(&bc)
	if bc.Status != constants.BootCodeExpired {
		t.Fatalf("[DB boot_codes code=%s] 过期码应幂等置为 expired，got %s（过期标记问题）", code, bc.Status)
	}
	env.assertStationIdle(st.ID, "过期码被拒绝后")
	env.assertNoSession(res.ID, "过期码被拒绝后")
}

// TestVerifyCancelledCode_Rejected 会员重新生成码后，旧码作废，扫旧码必须拒绝。
func TestVerifyCancelledCode_Rejected(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("A区-04", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	oldCode, _ := env.generateCode(memberToken, res.ID)
	newCode, _ := env.generateCode(memberToken, res.ID) // 旧码自动 cancelled
	if oldCode == newCode {
		t.Fatal("[POST /boot-codes] 重新生成的码不应与旧码相同（动态码刷新问题）")
	}
	var old model.BootCode
	env.db.Where("code = ?", oldCode).First(&old)
	if old.Status != constants.BootCodeCancelled {
		t.Fatalf("[DB boot_codes code=%s] 旧码应为 cancelled，got %s（旧码作废问题）", oldCode, old.Status)
	}
	env.assertVerify(staffToken, oldCode, 0, http.StatusConflict, constants.CodeBootExpired)
	// 新码仍可正常核销
	env.assertVerify(staffToken, newCode, 0, http.StatusOK, constants.CodeOK)
}

// TestVerifyStationMismatch_Rejected 扫码携带的机位与预约机位不一致时拒绝。
func TestVerifyStationMismatch_Rejected(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("A区-05", 8, constants.StationIdle)
	other := env.createStation("A区-06", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	code, _ := env.generateCode(memberToken, res.ID)
	env.assertVerify(staffToken, code, other.ID, http.StatusConflict, constants.CodeBootMismatch)
	env.assertStationIdle(st.ID, "机位不一致被拒绝后")
	env.assertNoSession(res.ID, "机位不一致被拒绝后")
	var bc model.BootCode
	env.db.Where("code = ?", code).First(&bc)
	if bc.Status != constants.BootCodeActive {
		t.Fatalf("[DB boot_codes code=%s] 校验失败应保持 active，got %s", code, bc.Status)
	}
}

// TestGenerate_CrossMemberAndRBAC 越权生成：他人预约、店员/管理员、未登录均被拒绝。
func TestGenerate_CrossMemberAndRBAC(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("A区-07", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID}) // 属于 it_member
	otherRes := env.createReservation(reservationOption{UserID: u.Poor.ID, StationID: st.ID,
		StartAt: time.Now().Add(30 * time.Minute), EndAt: time.Now().Add(2 * time.Hour)})

	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")
	adminToken := env.login("it_admin")

	// 会员为他人的预约生成码：403
	status, body := env.do("POST", "/api/v1/boot-codes", memberToken, map[string]any{"reservation_id": otherRes.ID})
	if status != http.StatusForbidden || body.Code != constants.CodeForbidden {
		t.Fatalf("[POST /boot-codes res=%d] 会员越权生成 want 403/%d got %d/%d msg=%q（归属校验问题）",
			otherRes.ID, constants.CodeForbidden, status, body.Code, body.Message)
	}
	// 店员、管理员走 RBAC 被拒：403
	for name, token := range map[string]string{"staff": staffToken, "admin": adminToken} {
		status, body = env.do("POST", "/api/v1/boot-codes", token, map[string]any{"reservation_id": res.ID})
		if status != http.StatusForbidden || body.Code != constants.CodeForbidden {
			t.Fatalf("[POST /boot-codes role=%s] 非会员角色生成 want 403 got %d/%d msg=%q（RBAC 问题）",
				name, status, body.Code, body.Message)
		}
	}
	// 未登录：401
	status, _ = env.do("POST", "/api/v1/boot-codes", "", map[string]any{"reservation_id": res.ID})
	if status != http.StatusUnauthorized {
		t.Fatalf("[POST /boot-codes] 未登录 want 401 got %d（认证中间件问题）", status)
	}
	// 被拒后不应产生开机码
	var cnt int64
	env.db.Model(&model.BootCode{}).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("[DB boot_codes] 越权失败不应留下开机码，got %d 条", cnt)
	}
}

// TestVerify_RoleMismatch 非店员/管理员核销：会员 403、未登录 401。
func TestVerify_RoleMismatch(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("B区-01", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")

	code, _ := env.generateCode(memberToken, res.ID)

	// 会员自己核销：RBAC 403
	status, body := env.do("POST", "/api/v1/boot-codes/verify", memberToken, map[string]any{"code": code})
	if status != http.StatusForbidden || body.Code != constants.CodeForbidden {
		t.Fatalf("[POST /boot-codes/verify role=member] want 403 got %d/%d msg=%q（RBAC 问题）", status, body.Code, body.Message)
	}
	// 未登录：401
	status, _ = env.do("POST", "/api/v1/boot-codes/verify", "", map[string]any{"code": code})
	if status != http.StatusUnauthorized {
		t.Fatalf("[POST /api/v1/boot-codes/verify] 未登录 want 401 got %d（认证中间件问题）", status)
	}
	env.assertStationIdle(st.ID, "角色不符核销被拒绝后")
}

// TestVerifyInvalidCode_NotFound 不存在/格式非法的码。
func TestVerifyInvalidCode_NotFound(t *testing.T) {
	env := newEnv(t)
	_ = env.seed()
	staffToken := env.login("it_staff")

	env.assertVerify(staffToken, "000000", 0, http.StatusNotFound, constants.CodeBootNotFound)

	// 非 6 位数字：参数校验 400
	status, body := env.do("POST", "/api/v1/boot-codes/verify", staffToken, map[string]any{"code": "12"})
	if status != http.StatusBadRequest || body.Code != constants.CodeValidation {
		t.Fatalf("[POST /boot-codes/verify code=12] want 400/%d got %d/%d（参数校验问题）", constants.CodeValidation, status, body.Code)
	}
}

// TestGenerate_ArrivalWindow 未到时间窗/预约过期不允许生成码。
func TestGenerate_ArrivalWindow(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("B区-02", 8, constants.StationIdle)
	tooEarly := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID,
		StartAt: time.Now().Add(2 * time.Hour), EndAt: time.Now().Add(3 * time.Hour)})
	expired := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID,
		StartAt: time.Now().Add(-3 * time.Hour), EndAt: time.Now().Add(-time.Hour)})
	memberToken := env.login("it_member")

	status, body := env.do("POST", "/api/v1/boot-codes", memberToken, map[string]any{"reservation_id": tooEarly.ID})
	if status != http.StatusConflict || body.Code != constants.CodeBootWindow {
		t.Fatalf("[POST /boot-codes] 未到时间窗 want 409/%d got %d/%d msg=%q（到店时间窗问题）", constants.CodeBootWindow, status, body.Code, body.Message)
	}
	status, body = env.do("POST", "/api/v1/boot-codes", memberToken, map[string]any{"reservation_id": expired.ID})
	if status != http.StatusConflict || body.Code != constants.CodeBootWindow {
		t.Fatalf("[POST /boot-codes] 预约过期 want 409/%d got %d/%d msg=%q（预约有效期问题）", constants.CodeBootWindow, status, body.Code, body.Message)
	}
}

// TestVerifyInsufficientBalance_Rollback 时长包与余额均不足时核销失败且事务整体回滚。
func TestVerifyInsufficientBalance_Rollback(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("B区-03", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Poor.ID, StationID: st.ID})
	poorToken := env.login("it_poor")
	staffToken := env.login("it_staff")

	code, _ := env.generateCode(poorToken, res.ID)
	env.assertVerify(staffToken, code, 0, http.StatusConflict, constants.CodeInsufficient)
	env.assertStationIdle(st.ID, "余额不足核销失败后")
	env.assertNoSession(res.ID, "余额不足核销失败后")
	var bc model.BootCode
	env.db.Where("code = ?", code).First(&bc)
	if bc.Status != constants.BootCodeActive {
		t.Fatalf("[DB boot_codes code=%s] 扣费失败回滚后码应仍为 active，got %s（事务原子性问题）", code, bc.Status)
	}
	var reservation model.Reservation
	env.db.First(&reservation, res.ID)
	if reservation.Status != constants.ReservationConfirmed {
		t.Fatalf("[DB reservations/%d] 扣费失败回滚后预约应仍为 confirmed，got %s", res.ID, reservation.Status)
	}
}

// TestVerify_PackageFirstDeduction 有足额时长包时优先扣包、不扣余额。
func TestVerify_PackageFirstDeduction(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("包厢区-01", 20, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	expire := time.Now().Add(30 * 24 * time.Hour)
	if err := env.db.Create(&model.UserPackage{
		UserID: u.Member.ID, PackageID: 1, PackageName: "3小时包",
		TotalHours: 3, RemainingHours: 3, ExpireAt: &expire, Status: "active",
	}).Error; err != nil {
		t.Fatalf("create user package: %v", err)
	}
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	code, _ := env.generateCode(memberToken, res.ID)
	v := env.assertVerify(staffToken, code, 0, http.StatusOK, constants.CodeOK)
	if v.BalanceCost != 0 {
		t.Fatalf("[POST /boot-codes/verify] 时长包足额时不应扣余额，got balanceCost=%f（扣包优先级问题）", v.BalanceCost)
	}
	if !(v.PackageHours > 1) || v.Balance != 100 {
		t.Fatalf("[POST /boot-codes/verify] 时长包扣减异常: packageHours=%f balance=%f", v.PackageHours, v.Balance)
	}
	var pkg model.UserPackage
	env.db.Where("user_id = ?", u.Member.ID).First(&pkg)
	if pkg.Status != "active" || !(pkg.RemainingHours < 3 && pkg.RemainingHours > 0.9) {
		t.Fatalf("[DB user_packages] 时长包扣减后剩余异常: remaining=%f status=%s", pkg.RemainingHours, pkg.Status)
	}
}

// TestVerify_StationBusy 机位被占用（using）时生成码、扫码核销均被拒绝。
func TestVerify_StationBusy(t *testing.T) {
	env := newEnv(t)
	u := env.seed()
	st := env.createStation("B区-04", 8, constants.StationIdle)
	res := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID})
	memberToken := env.login("it_member")
	staffToken := env.login("it_staff")

	// 先生成有效码，再把机位置为使用中（模拟临时被他人占用）
	code, _ := env.generateCode(memberToken, res.ID)
	if err := env.db.Model(&model.Station{}).Where("id = ?", st.ID).
		Update("status", constants.StationUsing).Error; err != nil {
		t.Fatalf("set station using: %v", err)
	}
	env.assertVerify(staffToken, code, 0, http.StatusConflict, constants.CodeStationBusy)
	env.assertNoSession(res.ID, "机位使用中核销失败后")
	var bc model.BootCode
	env.db.Where("code = ?", code).First(&bc)
	if bc.Status != constants.BootCodeActive {
		t.Fatalf("[DB boot_codes code=%s] 机位占用拒绝后码应保持 active，got %s", code, bc.Status)
	}

	// 机位一直 using 时，生成阶段直接拦截
	res2 := env.createReservation(reservationOption{UserID: u.Member.ID, StationID: st.ID,
		StartAt: time.Now().Add(24 * time.Hour), EndAt: time.Now().Add(25 * time.Hour)})
	status, body := env.do("POST", "/api/v1/boot-codes", memberToken, map[string]any{"reservation_id": res2.ID})
	if status == http.StatusOK {
		t.Fatalf("[POST /boot-codes] 机位 using 时生成不应成功（机位状态校验问题）, body=%q", body.Message)
	}
}

// ---- 数据库断言助手（失败信息带场景，便于区分接口还是页面/数据问题）----

func (e *env) assertStationIdle(stationID uint, scene string) {
	e.t.Helper()
	var st model.Station
	if err := e.db.First(&st, stationID).Error; err != nil {
		e.t.Fatalf("[DB stations/%d] %s 查询失败: %v", stationID, scene, err)
	}
	if st.Status != constants.StationIdle {
		e.t.Errorf("[DB stations/%d] %s 机位应保持 idle，got %s（事务回滚/状态机问题）", stationID, scene, st.Status)
	}
}

func (e *env) assertNoSession(reservationID uint, scene string) {
	e.t.Helper()
	var cnt int64
	if err := e.db.Model(&model.Session{}).Where("reservation_id = ?", reservationID).Count(&cnt).Error; err != nil {
		e.t.Fatalf("[DB sessions] %s 统计失败: %v", scene, err)
	}
	if cnt != 0 {
		e.t.Errorf("[DB sessions] %s 不应创建上机记录，got %d 条（失败分支未回滚）", scene, cnt)
	}
}
