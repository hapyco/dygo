// Package imports provides the durable CSV import foundation.
package imports

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/dygodata"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
)

const (
	coreApp       = "core"
	importEntity  = "import"
	rowEntity     = "import-row"
	processJob    = "process-import"
	maxImportRows = 50000
)

type authorizer interface {
	Can(context.Context, permissions.Request) error
}

type jobEnqueuer interface {
	Enqueue(context.Context, string, string, json.RawMessage, dygo.EnqueueOptions) (dygo.JobExecution, error)
}

// Target identifies the Entity populated by an import.
type Target struct {
	App    string `json:"app"`
	Entity string `json:"entity"`
}

// Info is the stable API representation of a queued import.
type Info struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Target    Target    `json:"target"`
	Status    string    `json:"status"`
	TotalRows int       `json:"total-rows"`
	Processed int       `json:"processed-rows"`
	Succeeded int       `json:"succeeded-rows"`
	Failed    int       `json:"failed-rows"`
	Rows      []RowInfo `json:"rows,omitempty"`
}

// RowInfo reports one durable import row outcome.
type RowInfo struct {
	RowNumber int    `json:"row-number"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	RecordID  int64  `json:"record-id,omitempty"`
}

// Service creates durable CSV imports.
type Service struct {
	queryer db.RecordQueryer
	store   db.RecordStore
	jobs    jobEnqueuer
	auth    authorizer
}

// NewService returns an importer backed by Core Import metadata.
func NewService(queryer db.RecordQueryer, jobs jobEnqueuer, auth authorizer) Service {
	return Service{queryer: queryer, store: db.NewRecordStoreWithHookPolicy(queryer, db.RecordMutationHooksNone), jobs: jobs, auth: auth}
}

// Start parses and persists a CSV import before enqueueing its processor.
func (s Service) Start(ctx context.Context, actor dygo.Actor, target Target, input io.Reader) (Info, error) {
	beginner, ok := s.queryer.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return s.start(ctx, actor, target, input)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return Info{}, err
	}
	defer tx.Rollback(ctx)
	jobs, err := dygodata.NewJobDataFromBeginner(tx)
	if err != nil {
		return Info{}, err
	}
	transactional := Service{queryer: tx, store: db.NewRecordStoreWithHookPolicy(tx, db.RecordMutationHooksNone), jobs: jobs, auth: s.auth}
	info, err := transactional.start(ctx, actor, target, input)
	if err != nil {
		return Info{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Info{}, err
	}
	return info, nil
}

func (s Service) start(ctx context.Context, actor dygo.Actor, target Target, input io.Reader) (Info, error) {
	if s.queryer == nil || s.jobs == nil {
		return Info{}, fmt.Errorf("import service is unavailable")
	}
	target.App = strings.TrimSpace(target.App)
	target.Entity = strings.TrimSpace(target.Entity)
	if target.App == "" || target.Entity == "" || input == nil {
		return Info{}, dygo.ActionError{Code: "invalid_request", Message: "CSV input and target Entity are required"}
	}
	if s.auth != nil {
		if err := s.auth.Can(ctx, permissions.Request{Actor: permissions.Actor(actor), Resource: permissions.Resource{Kind: permissions.ResourceEntity, App: target.App, Name: target.Entity}, Action: permissions.ActionCreate}); err != nil {
			return Info{}, err
		}
	}
	reader := csv.NewReader(input)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return Info{}, dygo.ActionError{Code: "invalid_request", Message: "CSV header is required"}
	}
	if err := validateHeaders(headers); err != nil {
		return Info{}, err
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	rows := make([]map[string]string, 0)
	for len(rows) < maxImportRows {
		values, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return Info{}, dygo.ActionError{Code: "invalid_request", Message: "CSV row is invalid"}
		}
		if len(values) != len(headers) {
			return Info{}, dygo.ActionError{Code: "invalid_request", Message: "CSV row does not match the header"}
		}
		row := make(map[string]string, len(headers))
		for i, header := range headers {
			row[header] = values[i]
		}
		rows = append(rows, row)
	}
	if _, err := reader.Read(); err != io.EOF {
		return Info{}, dygo.ActionError{Code: "invalid_request", Message: "CSV contains too many rows"}
	}
	headersJSON, _ := json.Marshal(headers)
	inputRecord := db.RecordInput{
		"app":            jsonString(target.App),
		"entity":         jsonString(target.Entity),
		"status":         jsonString("queued"),
		"actor":          jsonString(actor.Email),
		"headers":        headersJSON,
		"total-rows":     jsonInt(int64(len(rows))),
		"processed-rows": jsonInt(0),
		"succeeded-rows": jsonInt(0),
		"failed-rows":    jsonInt(0),
	}
	importRecord, err := s.store.SystemWriter().InsertReturningByIdentity(ctx, coreApp, importEntity, inputRecord, db.SystemMutationSilent)
	if err != nil {
		return Info{}, err
	}
	importID, ok := integer(importRecord["id"])
	if !ok {
		return Info{}, fmt.Errorf("import record id is invalid")
	}
	importName := stringValue(importRecord["name"])
	for index, row := range rows {
		data, _ := json.Marshal(row)
		_, err := s.store.SystemWriter().InsertReturningByIdentity(ctx, coreApp, rowEntity, db.RecordInput{
			"import":     jsonString(importName),
			"row-number": jsonInt(int64(index + 1)),
			"status":     jsonString("queued"),
			"data":       data,
		}, db.SystemMutationSilent)
		if err != nil {
			return Info{}, err
		}
	}
	payload, _ := json.Marshal(processPayload{ImportID: importID, ImportName: importName, Target: target, Actor: actor})
	if _, err := s.jobs.Enqueue(ctx, coreApp, processJob, payload, dygo.EnqueueOptions{IdempotencyKey: "import:" + importName}); err != nil {
		return Info{}, err
	}
	return Info{ID: importID, Name: importName, Target: target, Status: "queued", TotalRows: len(rows)}, nil
}

// Status returns one import and its row outcomes to its actor.
func (s Service) Status(ctx context.Context, actor dygo.Actor, id int64) (Info, error) {
	if id <= 0 {
		return Info{}, dygo.ActionError{Code: "invalid_request", Message: "import id must be positive"}
	}
	record, err := s.store.GetRecordByIdentity(ctx, coreApp, importEntity, id)
	if err != nil {
		return Info{}, err
	}
	if !actor.Administrator && stringValue(record["actor"]) != strings.TrimSpace(actor.Email) {
		return Info{}, dygo.ActionError{Code: "permission_denied", Message: "permission denied"}
	}
	info := Info{
		ID: id, Name: stringValue(record["name"]),
		Target: Target{App: stringValue(record["app"]), Entity: stringValue(record["entity"])},
		Status: stringValue(record["status"]), TotalRows: int(number(record["total-rows"])),
		Processed: int(number(record["processed-rows"])), Succeeded: int(number(record["succeeded-rows"])), Failed: int(number(record["failed-rows"])),
	}
	for offset := 0; ; offset += 2500 {
		rows, err := s.store.ListRecordsByIdentity(ctx, coreApp, rowEntity, db.RecordListParams{
			Limit: 2500, Offset: offset, Filters: []db.RecordFilter{{Field: "import", Operator: "eq", Value: info.Name}}, Sort: []db.RecordSort{{Field: "row-number"}},
		})
		if err != nil {
			return Info{}, err
		}
		for _, row := range rows.Records {
			info.Rows = append(info.Rows, RowInfo{RowNumber: int(number(row["row-number"])), Status: stringValue(row["status"]), Error: stringValue(row["error"]), RecordID: number(row["record-id"])})
		}
		if len(rows.Records) < 2500 {
			break
		}
	}
	return info, nil
}

// JobRegistrar registers the durable import processor used by dygo workers.
func JobRegistrar() dygo.JobRegistrar {
	return func(registry dygo.JobRegistry) error {
		return registry.RegisterJob(coreApp, processJob, process)
	}
}

type processPayload struct {
	ImportID   int64      `json:"import-id"`
	ImportName string     `json:"import-name"`
	Target     Target     `json:"target"`
	Actor      dygo.Actor `json:"actor"`
}

func process(ctx context.Context, job dygo.JobExecution) error {
	var payload processPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.ImportID <= 0 || payload.ImportName == "" || payload.Target.App == "" || payload.Target.Entity == "" {
		return fmt.Errorf("import job payload is invalid")
	}
	system := job.Records
	user, err := system.Get(ctx, coreApp, "user", payload.Actor.UserID)
	if err != nil {
		return err
	}
	enabled, _ := user["enabled"].(bool)
	if !enabled {
		return fmt.Errorf("import actor is disabled")
	}
	payload.Actor.Email = stringValue(user["email"])
	payload.Actor.Administrator, _ = user["administrator"].(bool)
	if _, err := system.Update(ctx, coreApp, importEntity, payload.ImportID, dygoRecordInput(map[string]any{"status": "running"})); err != nil {
		return err
	}
	actorRecords := job.Records.AsActor(payload.Actor)
	processed, succeeded, failed := 0, 0, 0
	offset := 0
	for {
		rows, err := system.List(ctx, coreApp, rowEntity, dygo.RecordListParams{Limit: 2500, Offset: offset, Filters: []dygo.RecordFilter{{Field: "import", Operator: "eq", Value: payload.ImportName}}, Sort: []dygo.RecordSort{{Field: "row-number"}}})
		if err != nil {
			return err
		}
		for _, row := range rows.Records {
			rowStatus := stringValue(row["status"])
			if rowStatus == "succeeded" {
				processed++
				succeeded++
				continue
			}
			if rowStatus == "failed" {
				processed++
				failed++
				continue
			}
			if rowStatus != "queued" {
				continue
			}
			rowID, ok := integer(row["id"])
			if !ok {
				continue
			}
			values, parseErr := importRowValues(row["data"])
			input := make(dygo.RecordInput, len(values))
			for field, value := range values {
				input[field] = jsonString(value)
			}
			processed++
			rowUpdate := map[string]any{"status": "succeeded"}
			switch {
			case parseErr != nil:
				failed++
				rowUpdate["status"] = "failed"
				rowUpdate["error"] = parseErr.Error()
			default:
				created, createErr := actorRecords.Create(ctx, payload.Target.App, payload.Target.Entity, input)
				if createErr != nil {
					failed++
					rowUpdate["status"] = "failed"
					rowUpdate["error"] = createErr.Error()
				} else {
					succeeded++
					if createdID, ok := integer(created["id"]); ok {
						rowUpdate["record-id"] = createdID
					}
				}
			}
			if _, err := system.Update(ctx, coreApp, rowEntity, rowID, dygoRecordInput(rowUpdate)); err != nil {
				return err
			}
		}
		if len(rows.Records) < 2500 {
			break
		}
		offset += len(rows.Records)
	}
	status := "succeeded"
	if failed > 0 {
		status = "failed"
	}
	_, err = system.Update(ctx, coreApp, importEntity, payload.ImportID, dygoRecordInput(map[string]any{"status": status, "processed-rows": processed, "succeeded-rows": succeeded, "failed-rows": failed}))
	return err
}

func validateHeaders(headers []string) error {
	seen := map[string]bool{}
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" || seen[header] {
			return dygo.ActionError{Code: "invalid_request", Message: "CSV headers must be unique and non-empty"}
		}
		seen[header] = true
	}
	if len(headers) == 0 {
		return dygo.ActionError{Code: "invalid_request", Message: "CSV header is required"}
	}
	return nil
}

func importRowValues(data any) (map[string]string, error) {
	switch raw := data.(type) {
	case json.RawMessage:
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("decode import row data: %w", err)
		}
		return values, nil
	case []byte:
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("decode import row data: %w", err)
		}
		return values, nil
	case map[string]any:
		values := make(map[string]string, len(raw))
		for field, value := range raw {
			values[field] = fmt.Sprint(value)
		}
		return values, nil
	default:
		return nil, nil
	}
}

func dygoRecordInput(values map[string]any) dygo.RecordInput {
	input := make(dygo.RecordInput, len(values))
	for key, value := range values {
		input[key], _ = json.Marshal(value)
	}
	return input
}

func jsonString(value string) json.RawMessage { data, _ := json.Marshal(value); return data }
func jsonInt(value int64) json.RawMessage     { data, _ := json.Marshal(value); return data }
func stringValue(value any) string            { valueString, _ := value.(string); return valueString }
func integer(value any) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case int:
		return int64(number), true
	case float64:
		return int64(number), true
	default:
		return 0, false
	}
}

func number(value any) int64 { number, _ := integer(value); return number }
