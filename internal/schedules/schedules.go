// Package schedules loads app-owned dygo Schedule metadata.
package schedules

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/jobs"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

const (
	// ScheduleSourceFile marks Schedules synced from apps/<app>/jobs/_schedules.yml.
	ScheduleSourceFile = "file"
	// ScheduleSourceStudio marks Schedules created from Studio-owned database metadata.
	ScheduleSourceStudio = "studio"
	// ScheduleSourceSystem marks Schedules created by dygo/system-owned code.
	ScheduleSourceSystem = "system"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// LoadedSchedule is one Schedule loaded from an owning app.
type LoadedSchedule struct {
	AppName  string
	AppDir   string
	Path     string
	Schedule Schedule
}

// File describes one app-level _schedules.yml document.
type File struct {
	Schedules []Schedule `yaml:"schedules"`
}

// Schedule describes one recurring Job trigger.
type Schedule struct {
	Name        string `yaml:"name"`
	Label       string `yaml:"label"`
	Description string `yaml:"description,omitempty"`
	Cron        string `yaml:"cron"`
	Timezone    string `yaml:"timezone"`
	Job         string `yaml:"job"`
	Enabled     *bool  `yaml:"enabled,omitempty"`
}

// EffectiveEnabled returns whether this Schedule should create future Job Executions.
func (s Schedule) EffectiveEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// JobRef returns the target app/job reference.
func (s Schedule) JobRef() (shape.AppRef, error) {
	ref, err := shape.ParseAppRef(s.Job)
	if err != nil {
		return shape.AppRef{}, fmt.Errorf("job must use <app>/<job>: %w", err)
	}
	return ref, nil
}

// NextRunAt returns the next scheduled occurrence after the given time in UTC.
func NextRunAt(cronExpr string, timezone string, after time.Time) (time.Time, error) {
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return time.Time{}, fmt.Errorf("timezone %q is invalid: %w", timezone, err)
	}
	schedule, err := cronParser.Parse("CRON_TZ=" + location.String() + " " + strings.TrimSpace(cronExpr))
	if err != nil {
		return time.Time{}, fmt.Errorf("cron %q is invalid: %w", cronExpr, err)
	}
	next := schedule.Next(after.UTC())
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron %q has no next run within five years", cronExpr)
	}
	return next.UTC(), nil
}

// Catalog loads Schedule metadata from discovered apps and Jobs.
type Catalog struct {
	apps []manifest.LoadedApp
	jobs []jobs.LoadedJob
}

// New returns a Schedule Catalog for apps and registered Jobs.
func New(apps []manifest.LoadedApp, loadedJobs []jobs.LoadedJob) Catalog {
	copiedApps := make([]manifest.LoadedApp, len(apps))
	copy(copiedApps, apps)
	copiedJobs := make([]jobs.LoadedJob, len(loadedJobs))
	copy(copiedJobs, loadedJobs)
	return Catalog{apps: copiedApps, jobs: copiedJobs}
}

// Discover loads app-level Schedule metadata files.
func (c Catalog) Discover() ([]LoadedSchedule, error) {
	var schedules []LoadedSchedule
	for _, app := range c.apps {
		discovered, err := c.discoverApp(app)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, discovered...)
	}
	sortSchedules(schedules)
	return schedules, nil
}

// Validate discovers Schedules and validates cross-file catalog rules.
func (c Catalog) Validate() ([]LoadedSchedule, error) {
	schedules, err := c.Discover()
	if err != nil {
		return nil, err
	}
	if err := validateCatalog(schedules, c.jobs); err != nil {
		return nil, err
	}
	return schedules, nil
}

func (c Catalog) discoverApp(app manifest.LoadedApp) ([]LoadedSchedule, error) {
	path := filepath.Join(app.Dir, filepath.FromSlash(shape.AppSchedulesFile))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat schedules for app %q from %s: %w", app.Manifest.Name, path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	file, err := LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load schedules for app %q from %s: %w", app.Manifest.Name, path, err)
	}
	loaded := make([]LoadedSchedule, 0, len(file.Schedules))
	for _, schedule := range file.Schedules {
		loaded = append(loaded, LoadedSchedule{
			AppName:  app.Manifest.Name,
			AppDir:   app.Dir,
			Path:     path,
			Schedule: schedule,
		})
	}
	return loaded, nil
}

// LoadFile reads and validates one app Schedule metadata file.
func LoadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read schedule metadata %s: %w", path, err)
	}
	file, err := Decode(data)
	if err != nil {
		return File{}, fmt.Errorf("load schedule metadata %s: %w", path, err)
	}
	return file, nil
}

// Decode decodes and validates one app Schedule metadata document.
func Decode(data []byte) (File, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return File{}, err
	}
	var file File
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil && err != io.EOF {
		return File{}, fmt.Errorf("decode schedule metadata: %w", err)
	}
	if file.Schedules == nil {
		file.Schedules = []Schedule{}
	}
	if err := file.Validate(); err != nil {
		return File{}, err
	}
	return file, nil
}

// Validate checks one app-level Schedule document without target Job context.
func (f File) Validate() error {
	var problems []string
	for index, schedule := range f.Schedules {
		prefix := fmt.Sprintf("schedules[%d]", index)
		validateSchedule(prefix, schedule, &problems)
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateSchedule(prefix string, schedule Schedule, problems *[]string) {
	if !fieldtype.IsName(strings.TrimSpace(schedule.Name)) {
		*problems = append(*problems, fmt.Sprintf("%s.name %q must be kebab-case", prefix, schedule.Name))
	}
	if strings.TrimSpace(schedule.Label) == "" {
		*problems = append(*problems, prefix+".label is required")
	}
	if strings.TrimSpace(schedule.Cron) == "" {
		*problems = append(*problems, prefix+".cron is required")
	} else if _, err := cronParser.Parse(strings.TrimSpace(schedule.Cron)); err != nil {
		*problems = append(*problems, fmt.Sprintf("%s.cron %q is invalid: %v", prefix, schedule.Cron, err))
	}
	if strings.TrimSpace(schedule.Timezone) == "" {
		*problems = append(*problems, prefix+".timezone is required")
	} else if _, err := time.LoadLocation(strings.TrimSpace(schedule.Timezone)); err != nil {
		*problems = append(*problems, fmt.Sprintf("%s.timezone %q is invalid: %v", prefix, schedule.Timezone, err))
	}
	if strings.TrimSpace(schedule.Job) == "" {
		*problems = append(*problems, prefix+".job is required")
	} else if _, err := schedule.JobRef(); err != nil {
		*problems = append(*problems, prefix+"."+err.Error())
	}
}

func validateCatalog(schedules []LoadedSchedule, loadedJobs []jobs.LoadedJob) error {
	var problems []string
	seen := map[string]LoadedSchedule{}
	jobsByKey := map[string]struct{}{}
	for _, loaded := range loadedJobs {
		jobsByKey[loaded.AppName+"\x00"+loaded.Job.Name] = struct{}{}
	}
	for _, loaded := range schedules {
		key := loaded.AppName + "\x00" + loaded.Schedule.Name
		if previous, ok := seen[key]; ok {
			problems = append(problems, fmt.Sprintf("%s: app %q schedule %q duplicates Schedule identity from %s", loaded.Path, loaded.AppName, loaded.Schedule.Name, previous.Path))
		}
		seen[key] = loaded
		ref, err := loaded.Schedule.JobRef()
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: app %q schedule %q has invalid target Job: %v", loaded.Path, loaded.AppName, loaded.Schedule.Name, err))
			continue
		}
		if _, ok := jobsByKey[ref.App+"\x00"+ref.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s: app %q schedule %q references missing Job %q", loaded.Path, loaded.AppName, loaded.Schedule.Name, loaded.Schedule.Job))
		}
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func sortSchedules(schedules []LoadedSchedule) {
	sort.SliceStable(schedules, func(i, j int) bool {
		if schedules[i].AppName != schedules[j].AppName {
			return schedules[i].AppName < schedules[j].AppName
		}
		if schedules[i].Schedule.Name != schedules[j].Schedule.Name {
			return schedules[i].Schedule.Name < schedules[j].Schedule.Name
		}
		return schedules[i].Path < schedules[j].Path
	})
}

// ValidationError reports one or more Schedule metadata problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "schedule metadata validation failed: " + strings.Join(e.Problems, "; ")
}

func rejectDuplicateKeys(data []byte) error {
	root, err := yamlmeta.Parse(data, "parse schedule metadata")
	if err != nil {
		return err
	}
	return yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate schedule metadata key %q at line %d, previously defined at line %d", duplicate.Location, duplicate.Line, duplicate.PreviousLine)
	})
}
