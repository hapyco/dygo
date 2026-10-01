package schedules

import (
	"github.com/hapyco/dygo/internal/app/manifest"
	"os"
	"path/filepath"
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

func TestCatalogValidatesJobCronSchedules(t *testing.T) {
	appDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(appDir, "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	apps := []manifest.LoadedApp{{Dir: appDir, Manifest: manifest.Manifest{Name: "sales"}}}
	loadedJobs := []jobs.LoadedJob{{AppName: "sales", Path: "jobs/report/job.yml", Job: jobs.Job{
		Name: "report", Label: "Report", Cron: "0 9 * * MON",
	}}}
	catalog := New(apps, loadedJobs)
	loaded, err := catalog.Validate()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("Validate() = %v, %v, want one Schedule", loaded, err)
	}
	schedule := loaded[0].Schedule
	if schedule.Name != "job-report" || schedule.Timezone != "UTC" || schedule.Job != "sales/report" || schedule.Cron != "0 9 * * MON" || !schedule.EffectiveEnabled() {
		t.Fatalf("generated Schedule = %+v", schedule)
	}
	if err := os.WriteFile(filepath.Join(appDir, "jobs", "_schedules.yml"), []byte(`schedules:
  - name: job-report
    label: Conflicting Report
    cron: "0 10 * * MON"
    timezone: UTC
    job: sales/report
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Validate(); err == nil || !strings.Contains(err.Error(), "duplicates Schedule identity") {
		t.Fatalf("Validate() error = %v, want duplicate Schedule rejection", err)
	}
	loadedJobs[0].Job.Cron = ""
	loaded, err = New(apps, loadedJobs).Validate()
	if err != nil || len(loaded) != 1 || loaded[0].Schedule.Cron != "0 10 * * MON" {
		t.Fatalf("Validate() after removing Job cron = %v, %v, want only explicit Schedule", loaded, err)
	}
}
