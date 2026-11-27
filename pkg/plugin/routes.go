package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
	"github.com/MiteshDhangar/beautify-alert/pkg/history"
	"github.com/MiteshDhangar/beautify-alert/pkg/notify"
	"github.com/MiteshDhangar/beautify-alert/pkg/routing"
	"github.com/MiteshDhangar/beautify-alert/pkg/templates"
)

const maxBodySize = 10 << 20

// Org roles, lowest first.
var roleRank = map[string]int{"Viewer": 1, "Editor": 2, "Admin": 3}

func (a *App) routes(mux *http.ServeMux) {
	// Grafana's webhook contact point posts here.
	mux.HandleFunc("POST /alert", a.requireRole("Editor", a.handleAlert))
	// Editor pages. The Templates page is for Editors, and a preview runs
	// template code, so Viewers cannot call it.
	mux.HandleFunc("GET /starters", a.handleStarters)
	mux.HandleFunc("POST /preview", a.requireRole("Editor", a.handlePreview))
	mux.HandleFunc("POST /test", a.requireRole("Editor", a.handleTest))
	mux.HandleFunc("GET /history", a.requireRole("Editor", a.handleHistory))
	mux.HandleFunc("GET /history/{id}", a.requireRole("Editor", a.handleHistoryEntry))
	mux.HandleFunc("POST /history/{id}/resend", a.requireRole("Editor", a.handleResend))
	mux.HandleFunc("POST /verify", a.requireRole("Admin", a.handleVerify))
}

// svc returns the notify service bound to the delivery log of the caller's
// Grafana organization.
func (a *App) svc(r *http.Request) *notify.Service {
	s := *a.service
	if s.Log == nil {
		s.Log = history.ForNamespace(backend.PluginConfigFromContext(r.Context()).Namespace)
	}
	return &s
}

// requireRole rejects callers whose org role is below min. It stops Viewers
// from sending email through the plugin.
func (a *App) requireRole(min string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := backend.UserFromContext(r.Context())
		role := ""
		if u != nil {
			role = u.Role
		}
		if roleRank[role] < roleRank[min] {
			writeError(w, http.StatusForbidden, fmt.Errorf("this action needs the %s role or higher in Grafana (yours: %q). For the webhook contact point, use a service account token with the Editor role", min, role))
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Error string `json:"error"`
	// Part is "subject" or "html" for template errors.
	Part string `json:"part,omitempty"`
}

func writeError(w http.ResponseWriter, status int, err error) {
	body := apiError{Error: err.Error()}
	var te *templates.Error
	if errors.As(err, &te) {
		body.Part = te.Part
	}
	writeJSON(w, status, body)
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	return nil
}

// handleAlert receives a notification from Grafana's webhook contact point.
// Query parameters ?template= and ?to= override alert labels.
func (a *App) handleAlert(w http.ResponseWriter, r *http.Request) {
	if a.configErr != nil {
		writeError(w, http.StatusServiceUnavailable, a.configErr)
		return
	}
	var n alert.Notification
	if err := decode(w, r, &n); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	q := r.URL.Query()
	res := a.svc(r).Handle(r.Context(), n, routing.Overrides{Template: q.Get("template"), Recipients: q.Get("to")})
	for _, e := range res.Emails {
		if e.OK {
			log.DefaultLogger.Info("Sent alert email", "template", e.Template, "to", strings.Join(e.To, ","), "alerts", e.Alerts)
		} else {
			log.DefaultLogger.Error("Alert email failed", "template", e.Template, "to", strings.Join(e.To, ","), "error", e.Error)
		}
		if !e.TemplateFound {
			log.DefaultLogger.Warn("Alert named a template that does not exist; used the default template", "template", e.Template)
		}
	}
	status := http.StatusOK
	if res.Failed() {
		// Grafana's webhook does not retry, but a non-2xx status marks the
		// notification as failed in Grafana's own notification history.
		status = http.StatusBadGateway
	}
	writeJSON(w, status, res)
}

type starterResponse struct {
	Starters []templates.Starter `json:"starters"`
	Sample   alert.Notification  `json:"sample"`
	Routing  config.Routing      `json:"routing"`
}

func (a *App) handleStarters(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, starterResponse{
		Starters: templates.Starters(),
		Sample:   alert.Sample(time.Now()),
		Routing:  a.settings.Routing,
	})
}

type previewRequest struct {
	Template config.Template     `json:"template"`
	Sample   *alert.Notification `json:"sample,omitempty"`
	// Status "resolved" previews the sample with every alert resolved.
	Status string `json:"status,omitempty"`
	To     string `json:"to,omitempty"`
}

func sampleFor(req previewRequest) alert.Notification {
	n := alert.Sample(time.Now())
	if req.Sample != nil {
		n = *req.Sample
	}
	if req.Status == alert.StatusResolved {
		now := time.Now()
		for i := range n.Alerts {
			if n.Alerts[i].Status != alert.StatusResolved {
				n.Alerts[i].Status = alert.StatusResolved
				n.Alerts[i].EndsAt = now.Truncate(time.Minute)
			}
		}
		n = n.Subset(n.Alerts)
	}
	return n
}

var errBusy = errors.New("too many previews and test emails are rendering right now; try again in a few seconds")

// handlePreview renders an unsaved template with sample data.
func (a *App) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !acquireRenderSlot() {
		writeError(w, http.StatusTooManyRequests, errBusy)
		return
	}
	defer releaseRenderSlot()
	out, err := templates.Render(req.Template, sampleFor(req))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleTest renders an unsaved template with sample data and emails it.
func (a *App) handleTest(w http.ResponseWriter, r *http.Request) {
	if a.configErr != nil {
		writeError(w, http.StatusServiceUnavailable, a.configErr)
		return
	}
	var req previewRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ns := backend.PluginConfigFromContext(r.Context()).Namespace
	if ok, wait := testLimiter.allow(ns, time.Now()); !ok {
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("this organization sent %d test emails in the last %s; try again in %s", testEmailLimit, testEmailWindow, wait.Round(time.Second)))
		return
	}
	if !acquireRenderSlot() {
		writeError(w, http.StatusTooManyRequests, errBusy)
		return
	}
	defer releaseRenderSlot()
	caller := notify.Caller{}
	if u := backend.UserFromContext(r.Context()); u != nil {
		caller = notify.Caller{Email: u.Email, Admin: roleRank[u.Role] >= roleRank["Admin"]}
	}
	entry, err := a.svc(r).SendTest(r.Context(), req.Template, config.SplitAddresses(req.To), caller)
	if err != nil {
		var te *templates.Error
		status := http.StatusBadGateway
		switch {
		case errors.As(err, &te):
			status = http.StatusUnprocessableEntity
		case errors.Is(err, notify.ErrTestRecipient):
			status = http.StatusForbidden
		case errors.Is(err, config.ErrNotConfigured), strings.HasPrefix(err.Error(), "invalid recipient"), strings.HasPrefix(err.Error(), "no recipients"):
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, entry.Summary())
}

func (a *App) handleHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"entries": a.svc(r).Log.List(history.Capacity), "capacity": history.Capacity})
}

func (a *App) handleHistoryEntry(w http.ResponseWriter, r *http.Request) {
	e, ok := a.svc(r).Log.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("log entry not found; the log keeps recent emails only and is cleared when Grafana restarts"))
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (a *App) handleResend(w http.ResponseWriter, r *http.Request) {
	entry, err := a.svc(r).Resend(r.Context(), r.PathValue("id"))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, notify.ErrNotResendable) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, entry.Summary())
}

type verifyRequest struct {
	// Delivery settings to test before saving. The password is taken from
	// the saved settings when Password is empty.
	Delivery config.Delivery `json:"delivery"`
	Password string          `json:"password,omitempty"`
}

// handleVerify connects to the email provider with the given settings and
// authenticates, without sending.
func (a *App) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	d := req.Delivery
	d.SMTP.Password = req.Password
	if d.SMTP.Password == "" && d.SMTP.Username != "" && a.settings.Delivery.SMTP.Password != "" {
		// The saved password only ever goes to the saved server. Otherwise
		// anyone who can call verify could send it to a server they run.
		if !sameServer(d.SMTP, a.settings.Delivery.SMTP) {
			writeError(w, http.StatusBadRequest, errors.New("enter the password: the saved password is only used to verify the saved server with the same host, port, username and security"))
			return
		}
		d.SMTP.Password = a.settings.Delivery.SMTP.Password
	}
	s := config.Settings{Delivery: d}
	if err := s.CheckDelivery(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	sender, err := delivery.New(d)
	if err == nil {
		err = sender.Verify(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// sameServer reports whether a is the saved server b with the same login and
// no weaker security: the same TLS mode, and the certificate check kept on
// if it is on in b.
func sameServer(a, b config.SMTP) bool {
	port := func(s config.SMTP) int {
		if s.Port != 0 {
			return s.Port
		}
		switch s.Security {
		case config.SecurityTLS:
			return 465
		case config.SecurityNone:
			return 25
		default:
			return 587
		}
	}
	return strings.EqualFold(strings.TrimSpace(a.Host), strings.TrimSpace(b.Host)) &&
		port(a) == port(b) && a.Username == b.Username &&
		security(a) == security(b) && (!a.SkipVerify || b.SkipVerify)
}

func security(s config.SMTP) string {
	if s.Security == "" {
		return config.SecurityStartTLS
	}
	return s.Security
}
