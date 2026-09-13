// Package integration 扫码开机闭环集成测试：httptest + 真实 Gin 路由 + JWT/RBAC + 内存 SQLite。
//
// 运行：
//
//	cd backend/test/integration && go mod tidy && go test ./... -v
//
// 每个用例使用独立内存数据库，随进程退出自动消失，无外部依赖、无落盘残留，可连续重复运行。
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/esportsbar/backend/internal/constants"
	"github.com/esportsbar/backend/internal/handler"
	"github.com/esportsbar/backend/internal/middleware"
	"github.com/esportsbar/backend/internal/model"
	"github.com/esportsbar/backend/internal/repository"
	"github.com/esportsbar/backend/internal/router"
	"github.com/esportsbar/backend/internal/service"
	"github.com/esportsbar/backend/internal/util"
)

const (
	jwtSecret = "integration_test_secret"
	testPwd   = "test123456"
)

// env 单条用例的测试环境：独立内存库 + 完整服务装配 + httptest 服务器。
type env struct {
	t      *testing.T
	db     *gorm.DB
	server *httptest.Server
}

type users struct {
	Admin  *model.User
	Staff  *model.User
	Staff2 *model.User
	Member *model.User // 有余额 100
	Poor   *model.User // 无余额无时长包
}

// 每个用例使用独立内存库；envSeq 保证同进程 -count=N 重复运行时 DSN 仍唯一、互不残留。
var envSeq int64

func nextDSN(name string) string {
	n := atomic.AddInt64(&envSeq, 1)
	return fmt.Sprintf("file:boot_%d_%s?mode=memory&cache=shared", n, strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, strings.ToLower(name)))
}

// newEnv 为当前用例创建独立环境（数据互相隔离、进程结束即清理）。
func newEnv(t *testing.T) *env {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := gorm.Open(sqlite.Open(nextDSN(t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Station{}, &model.TimePackage{}, &model.UserPackage{},
		&model.Recharge{}, &model.PackageOrder{}, &model.Reservation{}, &model.Session{},
		&model.Tournament{}, &model.Team{}, &model.Registration{}, &model.Match{},
		&model.AuditLog{}, &model.BootCode{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	// 与 cmd/server/main.go 一致的装配链。
	userRepo := repository.NewUserRepository(db)
	stationRepo := repository.NewStationRepository(db)
	pkgRepo := repository.NewTimePackageRepository(db)
	userPkgRepo := repository.NewUserPackageRepository(db)
	rechargeRepo := repository.NewRechargeRepository(db)
	orderRepo := repository.NewPackageOrderRepository(db)
	resRepo := repository.NewReservationRepository(db)
	sessRepo := repository.NewSessionRepository(db)
	bootRepo := repository.NewBootCodeRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	authSvc := service.NewAuthService(userRepo, logger, jwtSecret, 86400)
	userSvc := service.NewUserService(userRepo, logger)
	stationSvc := service.NewStationService(stationRepo, logger)
	_ = service.NewTimePackageService(pkgRepo, logger)
	_ = service.NewRechargeService(userRepo, rechargeRepo, pkgRepo, userPkgRepo, orderRepo, logger)
	_ = service.NewReservationService(resRepo, stationSvc, db, logger)
	sessSvc := service.NewSessionService(sessRepo, stationSvc, userPkgRepo, userRepo, resRepo, db, logger)
	bootSvc := service.NewBootCodeService(bootRepo, resRepo, sessRepo, userRepo, userPkgRepo, stationSvc, sessSvc, db, logger)
	auditSvc := service.NewAuditService(auditRepo, logger)

	authH := handler.NewAuthHandler(authSvc, userSvc, logger)
	bootH := handler.NewBootCodeHandler(bootSvc, logger)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}) // 不可达，限流自动降级为进程内
	engine.Use(middleware.RequestID(), middleware.ErrorHandler(logger), middleware.RateLimit(6000, rdb),
		cors.New(cors.Config{AllowAllOrigins: true, AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}}))
	api := engine.Group("/api/v1")
	api.Use(middleware.Audit(auditSvc))
	router.RegisterAuth(api, authH, jwtSecret)
	router.RegisterBootCode(api, bootH, jwtSecret)

	ts := httptest.NewServer(engine)
	t.Cleanup(ts.Close)
	return &env{t: t, db: db, server: ts}
}

// seed 自建测试数据：三种角色用户 + 一台空闲机位。
func (e *env) seed() users {
	hash, err := util.HashPassword(testPwd)
	if err != nil {
		e.t.Fatalf("hash password: %v", err)
	}
	u := users{
		Admin:  &model.User{Username: "it_admin", Password: hash, Nickname: "管理员", Role: constants.RoleAdmin, Status: "active"},
		Staff:  &model.User{Username: "it_staff", Password: hash, Nickname: "店员甲", Role: constants.RoleStaff, Status: "active"},
		Staff2: &model.User{Username: "it_staff2", Password: hash, Nickname: "店员乙", Role: constants.RoleStaff, Status: "active"},
		Member: &model.User{Username: "it_member", Password: hash, Nickname: "会员甲", Role: constants.RoleMember, Status: "active", Balance: 100},
		Poor:   &model.User{Username: "it_poor", Password: hash, Nickname: "欠费会员", Role: constants.RoleMember, Status: "active", Balance: 0},
	}
	for _, x := range []*model.User{u.Admin, u.Staff, u.Staff2, u.Member, u.Poor} {
		if err := e.db.Create(x).Error; err != nil {
			e.t.Fatalf("create user %s: %v", x.Username, err)
		}
	}
	return u
}

func (e *env) createStation(name string, price float64, status string) *model.Station {
	st := &model.Station{Name: name, Area: "测试区", StationType: "seat", PricePerHour: price, Status: status}
	if err := e.db.Create(st).Error; err != nil {
		e.t.Fatalf("create station %s: %v", name, err)
	}
	return st
}

// reservationOption 预约构造选项。
type reservationOption struct {
	UserID    uint
	StationID uint
	StartAt   time.Time // 零值取 now-5m
	EndAt     time.Time // 零值取 now+2h
	Status    string    // 零值取 confirmed
}

func (e *env) createReservation(o reservationOption) *model.Reservation {
	if o.StartAt.IsZero() {
		o.StartAt = time.Now().Add(-5 * time.Minute)
	}
	if o.EndAt.IsZero() {
		o.EndAt = time.Now().Add(2 * time.Hour)
	}
	if o.Status == "" {
		o.Status = constants.ReservationConfirmed
	}
	res := &model.Reservation{UserID: o.UserID, StationID: o.StationID, StartTime: o.StartAt, EndTime: o.EndAt, Status: o.Status}
	if err := e.db.Create(res).Error; err != nil {
		e.t.Fatalf("create reservation: %v", err)
	}
	return res
}

// respBody 统一响应。
type respBody struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// do 发起请求（token 为空表示不带 JWT）。
func (e *env) do(method, path, token string, payload any) (int, respBody) {
	e.t.Helper()
	var reader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		e.t.Fatalf("[%s %s] build request: %v", method, path, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("[%s %s] request error: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var body respBody
	_ = json.Unmarshal(b, &body)
	return resp.StatusCode, body
}

// login 走真实 /auth/login 接口换取 JWT（同时验证认证链路）。
func (e *env) login(username string) string {
	e.t.Helper()
	status, body := e.do("POST", "/api/v1/auth/login", "", map[string]string{"username": username, "password": testPwd})
	if status != http.StatusOK {
		e.t.Fatalf("[POST /auth/login user=%s] want 200 got %d body=%s", username, status, body.Message)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body.Data, &data); err != nil || data.Token == "" {
		e.t.Fatalf("[POST /auth/login user=%s] token missing: %v raw=%s", username, err, string(body.Data))
	}
	return data.Token
}

// generateCode 会员通过 POST /boot-codes 生成动态码，返回 (code, expireAt)。
func (e *env) generateCode(token string, reservationID uint) (string, time.Time) {
	e.t.Helper()
	status, body := e.do("POST", "/api/v1/boot-codes", token, map[string]any{"reservation_id": reservationID})
	if status != http.StatusOK || body.Code != constants.CodeOK {
		e.t.Fatalf("[POST /boot-codes res=%d] want 200/code0 got %d/%d msg=%s", reservationID, status, body.Code, body.Message)
	}
	var data struct {
		Code     string    `json:"code"`
		ExpireAt time.Time `json:"expire_at"`
		Status   string    `json:"status"`
	}
	if err := json.Unmarshal(body.Data, &data); err != nil {
		e.t.Fatalf("[POST /boot-codes res=%d] decode: %v", reservationID, err)
	}
	return data.Code, data.ExpireAt
}

// verifyResult 核销成功 data。
type verifyResult struct {
	SessionID       uint    `json:"session_id"`
	Code            string  `json:"code"`
	ReservationID   uint    `json:"reservation_id"`
	UserID          uint    `json:"user_id"`
	Username        string  `json:"username"`
	StationID       uint    `json:"station_id"`
	StationName     string  `json:"station_name"`
	StartTime       string  `json:"start_time"`
	ReservedEndTime string  `json:"reserved_end_time"`
	BalanceCost     float64 `json:"balance_cost"`
	Balance         float64 `json:"balance"`
	PackageHours    float64 `json:"package_hours"`
	Status          string  `json:"status"`
}

// assertVerify 发起核销并断言 HTTP 与业务码；wantOK 时返回详情。
func (e *env) assertVerify(token, code string, stationID uint, wantHTTP, wantCode int) verifyResult {
	e.t.Helper()
	payload := map[string]any{"code": code}
	if stationID > 0 {
		payload["station_id"] = stationID
	}
	status, body := e.do("POST", "/api/v1/boot-codes/verify", token, payload)
	if status != wantHTTP || body.Code != wantCode {
		e.t.Errorf("[POST /boot-codes/verify code=%s] want http=%d code=%d, got http=%d code=%d msg=%q（若 HTTP 不符是接口层问题，业务码不符是 service 校验问题）",
			code, wantHTTP, wantCode, status, body.Code, body.Message)
	}
	if wantCode != constants.CodeOK {
		return verifyResult{}
	}
	var v verifyResult
	if err := json.Unmarshal(body.Data, &v); err != nil {
		e.t.Errorf("[POST /boot-codes/verify code=%s] decode detail: %v", code, err)
	}
	return v
}
