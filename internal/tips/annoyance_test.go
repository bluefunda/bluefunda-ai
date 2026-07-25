package tips

import (
	"testing"
	"time"

	tipcatalog "github.com/bluefunda/tipcatalog"
)

func TestBudgetAllows_InvocationFloor(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := AnnoyanceState{Invocations: invocationBudget - 1}
	if budgetAllows(s, at) {
		t.Fatal("expected budget to deny: below the invocation floor")
	}
	s.Invocations = invocationBudget
	if !budgetAllows(s, at) {
		t.Fatal("expected budget to allow: at the invocation floor")
	}
}

func TestBudgetAllows_DailyCap(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := AnnoyanceState{Invocations: invocationBudget, ShownDay: dayKey(at), ShownToday: dailyBudget}
	if budgetAllows(s, at) {
		t.Fatal("expected budget to deny: daily cap reached")
	}

	s.ShownToday = dailyBudget - 1
	if !budgetAllows(s, at) {
		t.Fatal("expected budget to allow: under daily cap")
	}
}

func TestBudgetAllows_DailyCapResetsOnNewDay(t *testing.T) {
	yesterday := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	today := yesterday.Add(24 * time.Hour)
	s := AnnoyanceState{Invocations: invocationBudget, ShownDay: dayKey(yesterday), ShownToday: dailyBudget}
	if !budgetAllows(s, today) {
		t.Fatal("expected budget to allow: daily cap should reset on a new day")
	}
}

func TestDismissFamily_BackoffSchedule(t *testing.T) {
	withHome(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	origNow := now
	now = func() time.Time { return base }
	t.Cleanup(func() { now = origNow })

	wantDurations := []time.Duration{24 * time.Hour, 72 * time.Hour, 14 * 24 * time.Hour}
	for i, want := range wantDurations {
		until, err := DismissFamily("fam")
		if err != nil {
			t.Fatalf("DismissFamily (stage %d): %v", i, err)
		}
		if got := until.Sub(base); got != want {
			t.Fatalf("stage %d: dismissed until +%v, want +%v", i, got, want)
		}
	}

	// One more dismissal past the schedule's end must be permanent.
	until, err := DismissFamily("fam")
	if err != nil {
		t.Fatalf("DismissFamily (permanent): %v", err)
	}
	if until.Year() < 9999 {
		t.Fatalf("expected a permanent (far-future) dismissal, got %v", until)
	}

	// And stays permanent on further dismissals.
	until, err = DismissFamily("fam")
	if err != nil {
		t.Fatalf("DismissFamily (still permanent): %v", err)
	}
	if until.Year() < 9999 {
		t.Fatalf("expected dismissal to remain permanent, got %v", until)
	}
}

func TestDismissFamily_IndependentPerFamily(t *testing.T) {
	withHome(t)
	untilA, err := DismissFamily("fam-a")
	if err != nil {
		t.Fatalf("DismissFamily: %v", err)
	}
	untilB, err := DismissFamily("fam-b")
	if err != nil {
		t.Fatalf("DismissFamily: %v", err)
	}
	// Both dismissed for the first time (24h), independently of each other.
	if untilA.Sub(untilB) > time.Second || untilB.Sub(untilA) > time.Second {
		t.Fatalf("expected independent families to get the same first-stage backoff, got %v vs %v", untilA, untilB)
	}

	// Escalate only fam-a; fam-b's stage must be untouched.
	if _, err := DismissFamily("fam-a"); err != nil {
		t.Fatalf("DismissFamily: %v", err)
	}
	s, err := loadAnnoyance()
	if err != nil {
		t.Fatalf("loadAnnoyance: %v", err)
	}
	if s.Families["fam-b"].BackoffStage != 1 {
		t.Fatalf("fam-b backoff stage = %d, want 1 (untouched by fam-a's dismissal)", s.Families["fam-b"].BackoffStage)
	}
	if s.Families["fam-a"].BackoffStage != 2 {
		t.Fatalf("fam-a backoff stage = %d, want 2", s.Families["fam-a"].BackoffStage)
	}
}

func TestRecordShown_ResetsInvocationsAndIncrementsCounts(t *testing.T) {
	withHome(t)

	if err := withAnnoyanceLock(func(s *AnnoyanceState) { s.Invocations = 20 }); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := recordShown("tip-1"); err != nil {
		t.Fatalf("recordShown: %v", err)
	}

	s, err := loadAnnoyance()
	if err != nil {
		t.Fatalf("loadAnnoyance: %v", err)
	}
	if s.Invocations != 0 {
		t.Fatalf("Invocations = %d, want 0 (reset after showing)", s.Invocations)
	}
	if s.Tips["tip-1"].ShownCount != 1 {
		t.Fatalf("ShownCount = %d, want 1", s.Tips["tip-1"].ShownCount)
	}
	if s.ShownToday != 1 {
		t.Fatalf("ShownToday = %d, want 1", s.ShownToday)
	}
}

func TestRecordShown_RetiresAfterThreeShows(t *testing.T) {
	withHome(t)

	for i := 0; i < retireAfterShows; i++ {
		if err := recordShown("tip-1"); err != nil {
			t.Fatalf("recordShown: %v", err)
		}
	}
	s, err := loadAnnoyance()
	if err != nil {
		t.Fatalf("loadAnnoyance: %v", err)
	}
	if !isRetired(s, "tip-1") {
		t.Fatal("expected tip-1 to be retired after 3 shows")
	}

	tips := []tipcatalog.Tip{{ID: "tip-1"}, {ID: "tip-2"}}
	filtered := filterRetired(tips, s)
	if len(filtered) != 1 || filtered[0].ID != "tip-2" {
		t.Fatalf("expected only tip-2 to survive filtering, got %v", filtered)
	}
}

func TestDisableEnable(t *testing.T) {
	withHome(t)

	s, err := loadAnnoyance()
	if err != nil {
		t.Fatalf("loadAnnoyance: %v", err)
	}
	if s.Disabled {
		t.Fatal("expected tips enabled by default")
	}

	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	s, err = loadAnnoyance()
	if err != nil {
		t.Fatalf("loadAnnoyance: %v", err)
	}
	if !s.Disabled {
		t.Fatal("expected Disabled=true after Disable()")
	}

	if err := Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	s, err = loadAnnoyance()
	if err != nil {
		t.Fatalf("loadAnnoyance: %v", err)
	}
	if s.Disabled {
		t.Fatal("expected Disabled=false after Enable()")
	}
}

func TestTipStatesFrom_PopulatesCooldownAndDismissal(t *testing.T) {
	lastShown := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dismissedUntil := lastShown.Add(24 * time.Hour)
	ann := AnnoyanceState{
		Tips:     map[string]tipStat{"t1": {LastShownAt: lastShown}},
		Families: map[string]familyState{"fam": {DismissedUntil: dismissedUntil}},
	}
	tips := []tipcatalog.Tip{{ID: "t1", Family: "fam"}}

	states := tipStatesFrom(ann, tips)
	got := states["t1"]
	if !got.LastShownAt.Equal(lastShown) {
		t.Fatalf("LastShownAt = %v, want %v", got.LastShownAt, lastShown)
	}
	if !got.DismissedUntil.Equal(dismissedUntil) {
		t.Fatalf("DismissedUntil = %v, want %v", got.DismissedUntil, dismissedUntil)
	}
}
