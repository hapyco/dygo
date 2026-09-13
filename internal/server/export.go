package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/recordquery"
)

func (h recordHandler) exportRecords(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "service_unavailable", Message: "record store is unavailable"}})
		return
	}
	if !h.authorize(w, r, chi.URLParam(r, "entity"), permissions.ActionExport) {
		return
	}
	params, err := recordListParams(r)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	params.Limit = recordquery.MaxLimit
	params.Offset = 0
	store, err := h.storeFor(r, chi.URLParam(r, "entity"), permissions.ActionExport)
	if err != nil {
		writePermissionError(w, err)
		return
	}
	records := []db.Record{}
	for {
		page, err := store.ListRecords(r.Context(), chi.URLParam(r, "entity"), params)
		if err != nil {
			writeRecordError(w, err)
			return
		}
		records = append(records, page.Records...)
		if len(page.Records) == 0 || len(records) >= page.Total || len(page.Records) < params.Limit {
			break
		}
		params.Offset += len(page.Records)
	}
	columns := csvColumns(records)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q.csv", chi.URLParam(r, "entity")))
	writer := csv.NewWriter(w)
	if err := writer.Write(columns); err != nil {
		return
	}
	for _, record := range records {
		row := make([]string, len(columns))
		for i, column := range columns {
			row[i] = csvValue(record[column])
		}
		if err := writer.Write(row); err != nil {
			return
		}
	}
	writer.Flush()
}

func csvColumns(records []db.Record) []string {
	seen := map[string]bool{}
	for _, record := range records {
		for key := range record {
			seen[key] = true
		}
	}
	columns := make([]string, 0, len(seen))
	for column := range seen {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return columns
}

func csvValue(value any) string {
	if value == nil {
		return ""
	}
	if raw, ok := value.(json.RawMessage); ok {
		return string(raw)
	}
	if data, ok := value.([]byte); ok && json.Valid(data) {
		return string(data)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
