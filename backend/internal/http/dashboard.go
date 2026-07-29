package api

import (
	"html/template"
	"net/http"
	"time"
)

var dashboardTemplate = template.Must(template.New("dashboard").Funcs(template.FuncMap{
	"value": func(value *string) string {
		if value == nil {
			return "unknown"
		}
		return *value
	},
	"timestamp": func(value *time.Time) string {
		if value == nil {
			return "never"
		}
		return value.UTC().Format(time.RFC3339)
	},
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pelican MC Router</title><style>body{font:16px system-ui,sans-serif;max-width:56rem;margin:3rem auto;padding:0 1rem;color:#172033}section{border:1px solid #d6dbe4;border-radius:.5rem;padding:1rem;margin:1rem 0}dl{display:grid;grid-template-columns:max-content 1fr;gap:.5rem 1rem}dt{font-weight:600}.ok{color:#087443}.bad{color:#a72c2c}</style></head>
<body><h1>Pelican MC Router</h1><p class="{{if .Ready}}ok{{else}}bad{{end}}">{{if .Ready}}Ready{{else}}Not ready{{end}}: {{.ReadinessReason}}</p>
<section><h2>Build</h2><dl><dt>Version</dt><dd>{{.Build.Version}}</dd><dt>Revision</dt><dd>{{.Build.Revision}}</dd></dl></section>
<section><h2>Reconciliation</h2><dl><dt>In progress</dt><dd>{{.Reconciliation.InProgress}}</dd><dt>Outcome</dt><dd>{{value .Reconciliation.LastOutcome}}</dd><dt>Last completed</dt><dd>{{timestamp .Reconciliation.LastCompletedAt}}</dd><dt>Consecutive failures</dt><dd>{{.Reconciliation.ConsecutiveFailures}}</dd><dt>Desired routes</dt><dd>{{.Reconciliation.RouteChanges.Desired}}</dd><dt>Created</dt><dd>{{.Reconciliation.RouteChanges.Created}}</dd><dt>Updated</dt><dd>{{.Reconciliation.RouteChanges.Updated}}</dd><dt>Deleted</dt><dd>{{.Reconciliation.RouteChanges.Deleted}}</dd></dl></section></body></html>`))

func (s *Server) dashboardPage(w http.ResponseWriter, r *http.Request) {
	if s.dashboard == nil {
		http.NotFound(w, r)
		return
	}
	snapshot, err := s.dashboard.Snapshot(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "dashboard status unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(w, snapshot); err != nil {
		return
	}
}
