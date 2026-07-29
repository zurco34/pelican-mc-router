package api

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/actioncontrol"
	"github.com/zurco34/pelican-mc-router/internal/dashboard"
	"github.com/zurco34/pelican-mc-router/internal/dashboardauth"
	"github.com/zurco34/pelican-mc-router/internal/operationalhistory"
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
<section><h2>Reconciliation</h2><dl><dt>In progress</dt><dd>{{.Reconciliation.InProgress}}</dd><dt>Outcome</dt><dd>{{value .Reconciliation.LastOutcome}}</dd><dt>Last completed</dt><dd>{{timestamp .Reconciliation.LastCompletedAt}}</dd><dt>Consecutive failures</dt><dd>{{.Reconciliation.ConsecutiveFailures}}</dd><dt>Desired routes</dt><dd>{{.Reconciliation.RouteChanges.Desired}}</dd><dt>Created</dt><dd>{{.Reconciliation.RouteChanges.Created}}</dd><dt>Updated</dt><dd>{{.Reconciliation.RouteChanges.Updated}}</dd><dt>Deleted</dt><dd>{{.Reconciliation.RouteChanges.Deleted}}</dd></dl>{{if .ManualReconciliationEnabled}}<button id="reconcile" type="button">Reconcile now</button><p id="reconcile-result" aria-live="polite"></p><script>document.getElementById("reconcile").addEventListener("click",async function(){const b=this;b.disabled=true;const r=document.getElementById("reconcile-result");try{const x=await fetch("/api/v1/dashboard/reconcile",{method:"POST",headers:{"X-Pelican-MC-Router-CSRF":"1"}});r.textContent=x.ok?"Reconciliation completed.":"Reconciliation unavailable."}catch(_){r.textContent="Reconciliation unavailable."}finally{b.disabled=false}});</script>{{end}}</section><section><h2>Recent activity</h2>{{if .Events}}<ul>{{range .Events}}<li>{{.OccurredAt.UTC.Format "2006-01-02T15:04:05Z07:00"}} — {{.Kind}}: {{.Outcome}} (desired {{.Desired}}, changed {{.Changed}})</li>{{end}}</ul>{{else}}<p>No recorded activity.</p>{{end}}</section></body></html>`))

type dashboardPageModel struct {
	dashboard.Snapshot
	ManualReconciliationEnabled bool
	Events                      []operationalhistory.Event
}

const dashboardCSRFHeader = "X-Pelican-MC-Router-CSRF"

func (s *Server) dashboardPage(w http.ResponseWriter, r *http.Request) {
	if s.dashboard == nil {
		http.NotFound(w, r)
		return
	}
	if s.dashboardAuth != nil {
		if err := s.dashboardAuth.Authorize(r.Context(), r); err != nil {
			dashboardAuthorizationError(w, err)
			return
		}
	}
	snapshot, err := s.dashboard.Snapshot(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "dashboard status unavailable")
		return
	}
	var events []operationalhistory.Event
	if s.operationalHistory != nil {
		events, err = s.operationalHistory.List(r.Context(), 10)
		if err != nil {
			writeJSONError(w, http.StatusServiceUnavailable, "dashboard status unavailable")
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(w, dashboardPageModel{
		Snapshot:                    snapshot,
		ManualReconciliationEnabled: s.dashboardRefresh != nil && s.dashboardAuth != nil,
		Events:                      events,
	}); err != nil {
		return
	}
}

type manualReconcileResponse struct {
	Reconciliation reconciliationStatusResponse `json:"reconciliation"`
}

func (s *Server) reconcileDashboard(w http.ResponseWriter, r *http.Request) {
	if s.dashboardRefresh == nil || s.dashboardAuth == nil {
		http.NotFound(w, r)
		return
	}
	if !s.allowAction(w, actioncontrol.ActionReconcile) {
		return
	}
	if r.Header.Get(dashboardCSRFHeader) != "1" {
		writeJSONError(w, http.StatusForbidden, "dashboard access denied")
		return
	}
	if err := s.dashboardAuth.AuthorizeOperator(r.Context(), r); err != nil {
		dashboardAuthorizationError(w, err)
		return
	}
	slog.Info("dashboard manual reconciliation requested")
	if err := s.dashboardRefresh.Refresh(r.Context()); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			slog.Info("dashboard manual reconciliation canceled")
			writeJSONError(w, http.StatusRequestTimeout, "reconciliation canceled")
			return
		}
		slog.Warn("dashboard manual reconciliation failed")
		writeJSONError(w, http.StatusServiceUnavailable, "reconciliation unavailable")
		return
	}
	slog.Info("dashboard manual reconciliation completed")
	writeJSON(w, http.StatusOK, manualReconcileResponse{
		Reconciliation: reconciliationResponse(s.reconciliationStatus.Snapshot()),
	})
}

func dashboardAuthorizationError(w http.ResponseWriter, err error) {
	status := http.StatusUnauthorized
	message := "dashboard authentication required"
	if errors.Is(err, dashboardauth.ErrForbidden) {
		status = http.StatusForbidden
		message = "dashboard access denied"
	}
	writeJSONError(w, status, message)
}
