// Package plugin wires Beautify Alert into Grafana: the app instance and its
// HTTP resource endpoints under /api/plugins/<id>/resources/.
package plugin

import (
	"context"
	"net/http"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
	"github.com/MiteshDhangar/beautify-alert/pkg/history"
	"github.com/MiteshDhangar/beautify-alert/pkg/notify"
)

var (
	_ backend.CallResourceHandler   = (*App)(nil)
	_ instancemgmt.InstanceDisposer = (*App)(nil)
	_ backend.CheckHealthHandler    = (*App)(nil)
)

// App is one app instance. Grafana creates a new one whenever the plugin
// settings are saved, so App never needs to watch for changes.
type App struct {
	backend.CallResourceHandler
	settings config.Settings
	service  *notify.Service
	// configErr is set when the saved settings cannot be used; endpoints that
	// need them report it.
	configErr error
}

// NewApp creates an App from the saved settings.
func NewApp(_ context.Context, s backend.AppInstanceSettings) (instancemgmt.Instance, error) {
	settings, err := config.Load(s)
	if err != nil {
		// Keep the plugin usable so the user can fix the settings.
		return newApp(config.Settings{}, nil, err), nil
	}
	sender, err := delivery.New(settings.Delivery)
	return newApp(settings, sender, err), nil
}

func newApp(settings config.Settings, sender delivery.Sender, configErr error) *App {
	a := &App{
		settings:  settings,
		configErr: configErr,
		service:   &notify.Service{Settings: settings, Sender: sender, Log: history.Default, Now: time.Now},
	}
	mux := http.NewServeMux()
	a.routes(mux)
	a.CallResourceHandler = httpadapter.New(mux)
	return a
}

// Dispose implements instancemgmt.InstanceDisposer.
func (a *App) Dispose() {}

// CheckHealth reports whether email delivery is configured.
func (a *App) CheckHealth(_ context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if a.configErr != nil {
		return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: a.configErr.Error()}, nil
	}
	if err := a.settings.CheckDelivery(); err != nil {
		return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: err.Error()}, nil
	}
	return &backend.CheckHealthResult{Status: backend.HealthStatusOk, Message: "Email delivery is configured"}, nil
}
