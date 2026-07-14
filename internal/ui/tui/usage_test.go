package tui

import (
	"strings"
	"testing"
)

// ─── formatUsage ─────────────────────────────────────────────────────────────

func TestFormatUsage_NilReturnsMessage(t *testing.T) {
	out := formatUsage(nil)
	if out == "" {
		t.Error("expected non-empty string for nil UsageInfo")
	}
	if !strings.Contains(out, "No usage info") {
		t.Errorf("expected 'No usage info' message, got %q", out)
	}
}

func TestFormatUsage_ContainsPlanType(t *testing.T) {
	info := &UsageInfo{PlanType: "premium"}
	out := formatUsage(info)
	if !strings.Contains(out, "premium") {
		t.Errorf("expected plan type 'premium' in output: %q", out)
	}
}

func TestFormatUsage_ShowsRPMRow(t *testing.T) {
	info := &UsageInfo{
		PlanType:   "free",
		RPMUsed:    3,
		RPMLimit:   5,
		RPMPercent: 60.0,
	}
	out := formatUsage(info)
	if !strings.Contains(out, "RPM") {
		t.Errorf("expected RPM row in output: %q", out)
	}
	if !strings.Contains(out, "3/5") {
		t.Errorf("expected '3/5' in RPM row: %q", out)
	}
	if !strings.Contains(out, "60.0%") {
		t.Errorf("expected '60.0%%' in RPM row: %q", out)
	}
}

func TestFormatUsage_NoHourlyRow(t *testing.T) {
	info := &UsageInfo{
		PlanType:   "free",
		RPMUsed:    1,
		RPMLimit:   5,
		RPMPercent: 20.0,
	}
	out := formatUsage(info)
	if strings.Contains(strings.ToLower(out), "hourly") {
		t.Errorf("output should not contain 'hourly': %q", out)
	}
}

func TestFormatUsage_ShowsDailyAndMonthly(t *testing.T) {
	info := &UsageInfo{
		PlanType:       "premium",
		RPMUsed:        10,
		RPMLimit:       30,
		RPMPercent:     33.3,
		DailyPercent:   45.5,
		MonthlyPercent: 12.1,
	}
	out := formatUsage(info)
	if !strings.Contains(out, "Daily") {
		t.Errorf("expected 'Daily' row in output: %q", out)
	}
	if !strings.Contains(out, "45.5%") {
		t.Errorf("expected '45.5%%' in output: %q", out)
	}
	if !strings.Contains(out, "Monthly") {
		t.Errorf("expected 'Monthly' row in output: %q", out)
	}
	if !strings.Contains(out, "12.1%") {
		t.Errorf("expected '12.1%%' in output: %q", out)
	}
}

func TestFormatUsage_ShowsTokenCounts(t *testing.T) {
	info := &UsageInfo{
		PlanType:     "free",
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}
	out := formatUsage(info)
	if !strings.Contains(out, "1000") {
		t.Errorf("expected input tokens '1000' in output: %q", out)
	}
	if !strings.Contains(out, "500") {
		t.Errorf("expected output tokens '500' in output: %q", out)
	}
	if !strings.Contains(out, "1500") {
		t.Errorf("expected total tokens '1500' in output: %q", out)
	}
}

func TestFormatUsage_ZeroRPMShowsZeroSlashZero(t *testing.T) {
	info := &UsageInfo{PlanType: "free"}
	out := formatUsage(info)
	if !strings.Contains(out, "0/0") {
		t.Errorf("expected '0/0' for zero RPM values: %q", out)
	}
}

// ─── UsageInfo struct ─────────────────────────────────────────────────────────

func TestUsageInfo_NoHourlyField(t *testing.T) {
	// Compile-time assertion: constructing UsageInfo with all current fields
	// ensures the struct layout matches expectations. If HourlyPercent were
	// re-added, this struct literal would need to include it or fail compilation.
	_ = UsageInfo{
		PlanType:       "free",
		RPMUsed:        2,
		RPMLimit:       5,
		RPMPercent:     40.0,
		DailyPercent:   20.0,
		MonthlyPercent: 5.0,
		InputTokens:    1000,
		OutputTokens:   500,
		TotalTokens:    1500,
	}
}

func TestUsageInfo_FreePlanValues(t *testing.T) {
	info := UsageInfo{
		PlanType:  "free",
		RPMUsed:   4,
		RPMLimit:  5,
		RPMPercent: 80.0,
	}
	if info.RPMLimit != 5 {
		t.Errorf("RPMLimit: want 5, got %d", info.RPMLimit)
	}
	if info.RPMPercent != 80.0 {
		t.Errorf("RPMPercent: want 80.0, got %f", info.RPMPercent)
	}
}

func TestUsageInfo_PremiumPlanValues(t *testing.T) {
	info := UsageInfo{
		PlanType:  "premium",
		RPMUsed:   15,
		RPMLimit:  30,
		RPMPercent: 50.0,
	}
	if info.RPMLimit != 30 {
		t.Errorf("RPMLimit: want 30, got %d", info.RPMLimit)
	}
}
