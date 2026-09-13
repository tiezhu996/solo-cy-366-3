package service

import (
	"testing"
	"time"

	"github.com/esportsbar/backend/internal/constants"
)

// TestCheckBootCodeStatus 开机码状态机表驱动测试。
func TestCheckBootCodeStatus(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.Local)
	cases := []struct {
		name     string
		status   string
		expireAt time.Time
		want     int
	}{
		{name: "active_valid", status: constants.BootCodeActive, expireAt: now.Add(time.Minute), want: constants.CodeOK},
		{name: "active_expired", status: constants.BootCodeActive, expireAt: now.Add(-time.Second), want: constants.CodeBootExpired},
		{name: "active_at_expire_boundary", status: constants.BootCodeActive, expireAt: now, want: constants.CodeBootExpired},
		{name: "used", status: constants.BootCodeUsed, expireAt: now.Add(time.Minute), want: constants.CodeBootUsed},
		{name: "expired_status", status: constants.BootCodeExpired, expireAt: now.Add(time.Minute), want: constants.CodeBootExpired},
		{name: "cancelled", status: constants.BootCodeCancelled, expireAt: now.Add(time.Minute), want: constants.CodeBootExpired},
		{name: "unknown_status", status: "hacked", expireAt: now.Add(time.Minute), want: constants.CodeBootNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkBootCodeStatus(tc.status, now, tc.expireAt); got != tc.want {
				t.Fatalf("checkBootCodeStatus(%s) = %d, want %d", tc.status, got, tc.want)
			}
		})
	}
}

// TestInArrivalWindow 到店时间窗表驱动测试。
func TestInArrivalWindow(t *testing.T) {
	start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.Local)
	end := start.Add(2 * time.Hour)
	cases := []struct {
		name        string
		now         time.Time
		wantBefore  bool
		wantExpired bool
	}{
		{name: "too_early", now: start.Add(-30 * time.Minute), wantBefore: true},
		{name: "window_early_boundary", now: start.Add(-BootArriveEarlyMinutes * time.Minute), wantBefore: false},
		{name: "within_window", now: start.Add(30 * time.Minute)},
		{name: "at_end", now: end},
		{name: "after_end", now: end.Add(time.Minute), wantExpired: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, expired := inArrivalWindow(tc.now, start, end)
			if before != tc.wantBefore || expired != tc.wantExpired {
				t.Fatalf("inArrivalWindow(%s) = (%v,%v), want (%v,%v)", tc.name, before, expired, tc.wantBefore, tc.wantExpired)
			}
		})
	}
}

// TestSplitBilling 时长包优先、余额兜底拆账表驱动测试。
func TestSplitBilling(t *testing.T) {
	cases := []struct {
		name          string
		neededHours   float64
		consumedHours float64
		pricePerHour  float64
		wantPackage   float64
		wantBalance   float64
	}{
		{name: "all_by_package", neededHours: 2, consumedHours: 2, pricePerHour: 8, wantPackage: 2, wantBalance: 0},
		{name: "package_then_balance", neededHours: 2, consumedHours: 1, pricePerHour: 8, wantPackage: 1, wantBalance: 8},
		{name: "all_by_balance", neededHours: 2, consumedHours: 0, pricePerHour: 8, wantPackage: 0, wantBalance: 16},
		{name: "free_station", neededHours: 2, consumedHours: 0, pricePerHour: 0, wantPackage: 0, wantBalance: 0},
		{name: "package_more_than_needed", neededHours: 1, consumedHours: 1, pricePerHour: 20, wantPackage: 1, wantBalance: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkg, bal := splitBilling(tc.neededHours, tc.consumedHours, tc.pricePerHour)
			if pkg != tc.wantPackage || bal != tc.wantBalance {
				t.Fatalf("splitBilling() = (%f,%f), want (%f,%f)", pkg, bal, tc.wantPackage, tc.wantBalance)
			}
		})
	}
}

// TestRound2 金额保留两位小数。
func TestRound2(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{in: 1.236, want: 1.24},
		{in: 8.333333, want: 8.33},
		{in: 0, want: 0},
	}
	for _, tc := range cases {
		if got := round2(tc.in); got != tc.want {
			t.Fatalf("round2(%f) = %f, want %f", tc.in, got, tc.want)
		}
	}
}
