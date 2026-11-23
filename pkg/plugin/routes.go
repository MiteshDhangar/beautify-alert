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
	// Editor pages.
	mux.HandleFunc("GET /starters", a.handleStarters)
	mux.HandleFunc("POST /preview", a.handlePreview)
	mux.HandleFunc("POST /test", a.requireRole("Editor", a.handleTest))
	mux.HandleFunc("GET /history", a.requireRole("Editor", a.handleHistory))
	mux.HandleFunc("GET /history/{id}", a.requireRole("Editor", a.handleHistoryEntry))
	mux.HandleFunc("POST /history/{id}/resend", a.requireRole("Editor", a.handleResend))
	mux.HandleFunc("POST /verify", a.requireRole("Admin", a.handleVerify))
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
	res := a.service.Handle(r.Context(), n, routing.Overrides{Template: q.Get("template"), Recipients: q.Get("to")})
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
		// A non-2xx status makes Grafana retry the notification.
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

// handlePreview renders an unsaved template with sample data.
func (a *App) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
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
	entry, err := a.service.SendTest(r.Context(), req.Template, config.SplitAddresses(req.To))
	if err != nil {
		var te *templates.Error
		status := http.StatusBadGateway
		if errors.As(err, &te) {
			status = http.StatusUnprocessableEntity
		} else if errors.Is(err, config.ErrNotConfigured) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, entry.Summary())
}

func (a *App) handleHistory(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"entries": a.service.Log.List(history.Capacity), "capacity": history.Capacity})
}

func (a *App) handleHistoryEntry(w http.ResponseWriter, r *http.Request) {
	e, ok := a.service.Log.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("log entry not found; the log keeps recent emails only and is cleared when Grafana restarts"))
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (a *App) handleResend(w http.ResponseWriter, r *http.Request) {
	entry, err := a.service.Resend(r.Context(), r.PathValue("id"))
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
	if d.SMTP.Password == "" && d.SMTP.Username == a.settings.Delivery.SMTP.Username {
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
