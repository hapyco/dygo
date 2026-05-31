package schedules

import (
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/jobs"
)

func TestDecodeValidScheduleFile(t *testing.T) {
	file, err := Decode([]byte(`
schedules:
  - name: weekly-report
    label: Weekly Report
    cron: "0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
`))
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if len(file.Schedules) != 1 {
		t.Fatalf("schedules count = %d, want 1", len(file.Schedules))
	}
	schedule := file.Schedules[0]
	if schedule.Name != "weekly-report" || schedule.Label != "Weekly Report" || schedule.Cron != "0 9 * * MON" || schedule.Timezone != "Asia/Karachi" || schedule.Job != "sales/send-weekly-report" {
		t.Fatalf("schedule = %+v, want decoded metadata", schedule)
	}
	if !schedule.EffectiveEnabled() {
		t.Fatal("EffectiveEnabled() = false, want default true")
	}
}

func TestDecodeRejectsUnknownPayloadField(t *testing.T) {
	_, err := Decode([]byte(`
schedules:
  - name: weekly-report
    label: Weekly Report
    cron: "0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
    payload:
      report: weekly
`))
	if err == nil || !strings.Contains(err.Error(), "field payload not found") {
		t.Fatalf("Decode() error = %v, want unknown payload field", err)
	}
}

func TestDecodeRejectsInvalidCron(t *testing.T) {
	_, err := Decode([]byte(`
schedules:
  - name: weekly-report
    label: Weekly Report
    cron: "* * * * * *"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
`))
	if err == nil || !strings.Contains(err.Error(), "expected exactly 5 fields") {
		t.Fatalf("Decode() error = %v, want invalid 6-field cron", err)
	}
}

func TestDecodeRejectsCronTimezonePrefix(t *testing.T) {
	_, err := Decode([]byte(`
schedules:
  - name: weekly-report
    label: Weekly Report
    cron: "CRON_TZ=UTC 0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
`))
	if err == nil || !strings.Contains(err.Error(), "must not include CRON_TZ or TZ") {
		t.Fatalf("Decode() error = %v, want cron timezone prefix rejection", err)
	}
}

func TestDecodeRejectsPaddedScheduleName(t *testing.T) {
	_, err := Decode([]byte(`
schedules:
  - name: " weekly-report "
    label: Weekly Report
    cron: "0 9 * * MON"
    timezone: Asia/Karachi
    job: sales/send-weekly-report
`))
	if err == nil || !strings.Contains(err.Error(), `name " weekly-report " must be kebab-case`) {
		t.Fatalf("Decode() error = %v, want padded name rejection", err)
	}
}

func TestNextRunAtUsesScheduleTimezone(t *testing.T) {
	after := time.Date(2026, 6, 1, 3, 59, 0, 0, time.UTC)
	next, err := NextRunAt("0 9 * * MON", "Asia/Karachi", after)
	if err != nil {
		t.Fatalf("NextRunAt() error = %v, want nil", err)
	}
	want := time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("NextRunAt() = %s, want %s", next, want)
	}
}

func TestNextRunAtRejectsCronTimezonePrefix(t *testing.T) {
	_, err := NextRunAt("TZ=UTC 0 9 * * MON", "Asia/Karachi", time.Date(2026, 6, 1, 3, 59, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "must not include CRON_TZ or TZ") {
		t.Fatalf("NextRunAt() error = %v, want timezone prefix rejection", err)
	}
}

func TestCatalogRejectsMissingTargetJob(t *testing.T) {
	catalog := New(nil, []jobs.LoadedJob{
		{AppName: "sales", Job: jobs.Job{Name: "send-invoice"}},
	})
	loaded := []LoadedSchedule{
		{
			AppName: "sales",
			Path:    "apps/sales/jobs/_schedules.yml",
			Schedule: Schedule{
				Name:     "weekly-report",
				Label:    "Weekly Report",
				Cron:     "0 9 * * MON",
				Timezone: "Asia/Karachi",
				Job:      "sales/send-weekly-report",
			},
		},
	}
	err := validateCatalog(loaded, catalog.jobs)
	if err == nil || !strings.Contains(err.Error(), `references missing Job "sales/send-weekly-report"`) {
		t.Fatalf("validateCatalog() error = %v, want missing target job", err)
	}
}

func TestCatalogSortsSchedules(t *testing.T) {
	schedules := []LoadedSchedule{
		{AppName: "zeta", Schedule: Schedule{Name: "one"}},
		{AppName: "alpha", Schedule: Schedule{Name: "two"}},
		{AppName: "alpha", Schedule: Schedule{Name: "one"}},
	}
	sortSchedules(schedules)
	got := []string{schedules[0].AppName + "/" + schedules[0].Schedule.Name, schedules[1].AppName + "/" + schedules[1].Schedule.Name, schedules[2].AppName + "/" + schedules[2].Schedule.Name}
	want := []string{"alpha/one", "alpha/two", "zeta/one"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted schedules = %v, want %v", got, want)
		}
	}
}
