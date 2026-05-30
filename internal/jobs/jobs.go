// Package jobs loads app-owned dygo Job metadata.
package jobs

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
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"gopkg.in/yaml.v3"
)

const (
	defaultRetryInitialDelay = "10s"
	defaultRetryMaxDelay     = "5m"
)

const (
	// JobSourceFile marks Jobs synced from apps/<app>/jobs/<job>/job.yml.
	JobSourceFile = "file"
	// JobSourceStudio marks Jobs created from Studio-owned database metadata.
	JobSourceStudio = "studio"
	// JobSourceSystem marks Jobs created by dygo/system-owned code.
	JobSourceSystem = "system"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// LoadedJob is one Job loaded from an owning app.
type LoadedJob struct {
	AppName string
	AppDir  string
	Path    string
	Job     Job
}

// Job describes one background Job definition.
type Job struct {
	Name        string `yaml:"-"`
	Label       string `yaml:"label"`
	Description string `yaml:"description,omitempty"`
	Queue       string `yaml:"queue,omitempty"`
	Timeout     string `yaml:"timeout"`
	Retry       *Retry `yaml:"retry,omitempty"`
}

// Retry describes retry behavior for one Job.
type Retry struct {
	Attempts     int    `yaml:"attempts"`
	InitialDelay string `yaml:"initial-delay,omitempty"`
	MaxDelay     string `yaml:"max-delay,omitempty"`
}

// EffectiveQueue returns the registered queue name for this Job.
func (j Job) EffectiveQueue() string {
	queue := strings.TrimSpace(j.Queue)
	if queue == "" {
		return queues.DefaultName
	}
	return queue
}

// MaxAttempts returns the total allowed attempts for executions of this Job.
func (j Job) MaxAttempts() int {
	if j.Retry == nil {
		return 1
	}
	return j.Retry.Attempts
}

// EffectiveRetry returns retry settings with defaults applied.
func (j Job) EffectiveRetry() *Retry {
	if j.Retry == nil {
		return nil
	}
	retry := *j.Retry
	if strings.TrimSpace(retry.InitialDelay) == "" {
		retry.InitialDelay = defaultRetryInitialDelay
	}
	if strings.TrimSpace(retry.MaxDelay) == "" {
		retry.MaxDelay = defaultRetryMaxDelay
	}
	return &retry
}

// Catalog loads Job metadata from discovered apps.
type Catalog struct {
	apps   []manifest.LoadedApp
	queues queues.Config
}

// New returns a Job Catalog for apps and registered queues.
func New(apps []manifest.LoadedApp, queueConfig queues.Config) Catalog {
	copied := make([]manifest.LoadedApp, len(apps))
	copy(copied, apps)
	return Catalog{apps: copied, queues: queueConfig}
}

// Discover loads Job metadata files.
func (c Catalog) Discover() ([]LoadedJob, error) {
	var jobs []LoadedJob
	for _, app := range c.apps {
		discovered, err := c.discoverApp(app)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, discovered...)
	}
	sortJobs(jobs)
	return jobs, nil
}

// Validate discovers Jobs and validates app-level Job catalog rules.
func (c Catalog) Validate() ([]LoadedJob, error) {
	jobs, err := c.Discover()
	if err != nil {
		return nil, err
	}
	if err := validateCatalog(jobs, c.queues); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (c Catalog) discoverApp(app manifest.LoadedApp) ([]LoadedJob, error) {
	jobsDir := filepath.Join(app.Dir, shape.AppJobsDir)
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read jobs for app %q from %s: %w", app.Manifest.Name, jobsDir, err)
	}

	var jobs []LoadedJob
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(jobsDir, entry.Name(), shape.JobMetadataFile)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat job metadata for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		job, err := LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load job for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		jobs = append(jobs, LoadedJob{
			AppName: app.Manifest.Name,
			AppDir:  app.Dir,
			Path:    path,
			Job:     job,
		})
	}
	return jobs, nil
}

// LoadFile reads and validates one Job metadata file.
func LoadFile(path string) (Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Job{}, fmt.Errorf("read job metadata %s: %w", path, err)
	}
	job, err := Decode(data)
	if err != nil {
		return Job{}, fmt.Errorf("load job metadata %s: %w", path, err)
	}
	name, err := jobNameFromPath(path)
	if err != nil {
		return Job{}, fmt.Errorf("load job metadata %s: %w", path, err)
	}
	job.Name = name
	if err := job.Validate(); err != nil {
		return Job{}, fmt.Errorf("load job metadata %s: %w", path, err)
	}
	return job, nil
}

// Decode decodes and validates one Job metadata document.
func Decode(data []byte) (Job, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return Job{}, err
	}
	var job Job
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&job); err != nil && err != io.EOF {
		return Job{}, fmt.Errorf("decode job metadata: %w", err)
	}
	if err := job.Validate(); err != nil {
		return Job{}, err
	}
	return job, nil
}

// Validate checks one Job definition without queue registry context.
func (j Job) Validate() error {
	var problems []string
	if strings.TrimSpace(j.Name) != "" && !fieldtype.IsName(j.Name) {
		problems = append(problems, fmt.Sprintf("job %q must be kebab-case", j.Name))
	}
	if strings.TrimSpace(j.Label) == "" {
		problems = append(problems, "label is required")
	}
	if queue := strings.TrimSpace(j.Queue); queue != "" && !fieldtype.IsName(queue) {
		problems = append(problems, fmt.Sprintf("queue %q must be kebab-case", j.Queue))
	}
	if _, err := positiveDuration(j.Timeout, "timeout"); err != nil {
		problems = append(problems, err.Error())
	}
	validateRetry(j.Retry, &problems)
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateCatalog(jobs []LoadedJob, queueConfig queues.Config) error {
	var problems []string
	seen := map[string]LoadedJob{}
	for _, loaded := range jobs {
		key := loaded.AppName + "\x00" + loaded.Job.Name
		if previous, ok := seen[key]; ok {
			problems = append(problems, fmt.Sprintf("%s: app %q job %q duplicates Job identity from %s", loaded.Path, loaded.AppName, loaded.Job.Name, previous.Path))
		}
		seen[key] = loaded
		queue := loaded.Job.EffectiveQueue()
		if !queueConfig.Has(queue) {
			problems = append(problems, fmt.Sprintf("%s: app %q job %q references unregistered queue %q", loaded.Path, loaded.AppName, loaded.Job.Name, queue))
		}
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateRetry(retry *Retry, problems *[]string) {
	if retry == nil {
		return
	}
	if retry.Attempts < 2 {
		*problems = append(*problems, "retry.attempts must be at least 2")
	}
	initialDelay, err := optionalPositiveDuration(retry.InitialDelay, defaultRetryInitialDelay, "retry.initial-delay")
	if err != nil {
		*problems = append(*problems, err.Error())
	}
	maxDelay, err := optionalPositiveDuration(retry.MaxDelay, defaultRetryMaxDelay, "retry.max-delay")
	if err != nil {
		*problems = append(*problems, err.Error())
	}
	if initialDelay > 0 && maxDelay > 0 && maxDelay < initialDelay {
		*problems = append(*problems, "retry.max-delay must be greater than or equal to retry.initial-delay")
	}
}

func positiveDuration(value string, name string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s %q must be a valid duration", name, value)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", name)
	}
	return duration, nil
}

func optionalPositiveDuration(value string, fallback string, name string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.ParseDuration(fallback)
	}
	return positiveDuration(value, name)
}

func jobNameFromPath(path string) (string, error) {
	if filepath.Base(path) != shape.JobMetadataFile {
		return "", fmt.Errorf("job metadata filename must be %s", shape.JobMetadataFile)
	}
	name := filepath.Base(filepath.Dir(path))
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("job folder name is required")
	}
	if !fieldtype.IsName(name) {
		return "", fmt.Errorf("job folder name %q must be kebab-case", name)
	}
	return name, nil
}

func sortJobs(jobs []LoadedJob) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].AppName != jobs[j].AppName {
			return jobs[i].AppName < jobs[j].AppName
		}
		if jobs[i].Job.Name != jobs[j].Job.Name {
			return jobs[i].Job.Name < jobs[j].Job.Name
		}
		return jobs[i].Path < jobs[j].Path
	})
}

// ValidationError reports one or more Job metadata problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "job metadata validation failed: " + strings.Join(e.Problems, "; ")
}

func rejectDuplicateKeys(data []byte) error {
	root, err := yamlmeta.Parse(data, "parse job metadata")
	if err != nil {
		return err
	}
	return yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate job metadata key %q at line %d, previously defined at line %d", duplicate.Location, duplicate.Line, duplicate.PreviousLine)
	})
}
