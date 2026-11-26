// Package alert holds the notification Grafana's webhook contact point sends,
// which is also the data every email template receives.
//
// Field names match Grafana's own notification templates, so a template
// written for Grafana ({{ .CommonLabels.alertname }}, {{ range .Alerts }})
// works here unchanged.
package alert

import (
	"sort"
	"time"
)

// KV is a set of labels or annotations. Templates read it like a map
// ({{ .Labels.severity }}) or in name order with SortedPairs.
type KV map[string]string

// Pair is one entry of a KV.
type Pair struct {
	Name  string
	Value string
}

// SortedPairs returns the entries sorted by name.
func (kv KV) SortedPairs() []Pair {
	pairs := make([]Pair, 0, len(kv))
	for k, v := range kv {
		pairs = append(pairs, Pair{Name: k, Value: v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Name < pairs[j].Name })
	return pairs
}

// Names returns the sorted keys.
func (kv KV) Names() []string {
	names := make([]string, 0, len(kv))
	for k := range kv {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Remove returns a copy of kv without the given keys. Templates use it to hide
// routing labels: {{ range (.Labels.Remove "email_template" "email_to").SortedPairs }}.
func (kv KV) Remove(keys ...string) KV {
	drop := make(map[string]bool, len(keys))
	for _, k := range keys {
		drop[k] = true
	}
	out := KV{}
	for k, v := range kv {
		if !drop[k] {
			out[k] = v
		}
	}
	return out
}

// Status values.
const (
	StatusFiring   = "firing"
	StatusResolved = "resolved"
)

// Alert is one alert instance in a notification.
type Alert struct {
	Status       string             `json:"status"`
	Labels       KV                 `json:"labels"`
	Annotations  KV                 `json:"annotations"`
	StartsAt     time.Time          `json:"startsAt"`
	EndsAt       time.Time          `json:"endsAt"`
	GeneratorURL string             `json:"generatorURL"`
	Fingerprint  string             `json:"fingerprint"`
	SilenceURL   string             `json:"silenceURL"`
	DashboardURL string             `json:"dashboardURL"`
	PanelURL     string             `json:"panelURL"`
	RuleUID      string             `json:"ruleUID,omitempty"`
	Values       map[string]float64 `json:"values"`
	ValueString  string             `json:"valueString"`
	ImageURL     string             `json:"imageURL,omitempty"`
	OrgID        *int64             `json:"orgId,omitempty"`
}

// IsFiring reports whether the alert is firing.
func (a Alert) IsFiring() bool { return a.Status == StatusFiring }

// Grafana raises these alerts itself when a rule's query fails or returns no
// data. They carry the rule's name in the "rulename" label.
const (
	AlertNameDatasourceError  = "DatasourceError"
	AlertNameDatasourceNoData = "DatasourceNoData"
)

// Problem is "query-error" or "no-data" for Grafana's own error alerts, else "".
func (a Alert) Problem() string {
	switch a.Labels["alertname"] {
	case AlertNameDatasourceError:
		return "query-error"
	case AlertNameDatasourceNoData:
		return "no-data"
	}
	return ""
}

// Name is the alert's name for people: the rule name, with "(query failed)"
// or "(no data)" for Grafana's error alerts, which are otherwise just named
// DatasourceError and DatasourceNoData.
func (a Alert) Name() string {
	name := a.Labels["alertname"]
	rule := a.Labels["rulename"]
	switch a.Problem() {
	case "query-error":
		if rule != "" {
			return rule + " (query failed)"
		}
	case "no-data":
		if rule != "" {
			return rule + " (no data)"
		}
	}
	return name
}

// Error is the error Grafana attached to a DatasourceError alert, if any.
func (a Alert) Error() string { return a.Annotations["Error"] }

// Alerts is a list of alerts with the helpers Grafana templates have.
type Alerts []Alert

// Firing returns the firing alerts.
func (as Alerts) Firing() Alerts { return as.withStatus(StatusFiring) }

// Resolved returns the resolved alerts.
func (as Alerts) Resolved() Alerts { return as.withStatus(StatusResolved) }

func (as Alerts) withStatus(status string) Alerts {
	out := Alerts{}
	for _, a := range as {
		if a.Status == status {
			out = append(out, a)
		}
	}
	return out
}

// Notification is the JSON body of Grafana's webhook contact point.
type Notification struct {
	Receiver          string `json:"receiver"`
	Status            string `json:"status"`
	Alerts            Alerts `json:"alerts"`
	GroupLabels       KV     `json:"groupLabels"`
	CommonLabels      KV     `json:"commonLabels"`
	CommonAnnotations KV     `json:"commonAnnotations"`
	ExternalURL       string `json:"externalURL"`
	Version           string `json:"version"`
	GroupKey          string `json:"groupKey"`
	TruncatedAlerts   int    `json:"truncatedAlerts"`
	OrgID             int64  `json:"orgId"`
	Title             string `json:"title"`
	State             string `json:"state"`
	Message           string `json:"message"`

	// Alerts dropped by Limit, by status. Not part of Grafana's payload.
	TruncatedFiring   int `json:"-"`
	TruncatedResolved int `json:"-"`
}

// FiringCount is the number of firing alerts, including any Limit dropped.
func (n Notification) FiringCount() int { return len(n.Alerts.Firing()) + n.TruncatedFiring }

// ResolvedCount is the number of resolved alerts, including any Limit dropped.
func (n Notification) ResolvedCount() int { return len(n.Alerts.Resolved()) + n.TruncatedResolved }

// Name is a name for the whole notification: the alerts' shared Name, else
// the common alertname label, else "".
func (n Notification) Name() string {
	if len(n.Alerts) == 0 {
		return n.CommonLabels["alertname"]
	}
	first := n.Alerts[0].Name()
	for _, a := range n.Alerts[1:] {
		if a.Name() != first {
			return n.CommonLabels["alertname"]
		}
	}
	return first
}

// Limit keeps at most max alerts, firing ones first, and adds the number
// dropped to TruncatedAlerts. max <= 0 keeps every alert.
func (n Notification) Limit(max int) Notification {
	if max <= 0 || len(n.Alerts) <= max {
		return n
	}
	kept := append(Alerts{}, n.Alerts.Firing()...)
	kept = append(kept, n.Alerts.Resolved()...)
	for _, a := range n.Alerts {
		if a.Status != StatusFiring && a.Status != StatusResolved {
			kept = append(kept, a)
		}
	}
	out := n
	for _, a := range kept[max:] {
		switch a.Status {
		case StatusFiring:
			out.TruncatedFiring++
		case StatusResolved:
			out.TruncatedResolved++
		}
	}
	out.TruncatedAlerts += len(kept) - max
	out.Alerts = kept[:max]
	return out
}

// Subset returns a copy of n that holds only alerts, with Status,
// CommonLabels and CommonAnnotations recomputed for them. The router uses it
// when one notification turns into several emails.
func (n Notification) Subset(alerts Alerts) Notification {
	out := n
	out.Alerts = alerts
	out.Status = StatusResolved
	if len(alerts.Firing()) > 0 {
		out.Status = StatusFiring
	}
	labels := make([]KV, len(alerts))
	annotations := make([]KV, len(alerts))
	for i, a := range alerts {
		labels[i] = a.Labels
		annotations[i] = a.Annotations
	}
	out.CommonLabels = common(labels)
	out.CommonAnnotations = common(annotations)
	return out
}

func common(sets []KV) KV {
	out := KV{}
	if len(sets) == 0 {
		return out
	}
	for k, v := range sets[0] {
		shared := true
		for _, s := range sets[1:] {
			if got, ok := s[k]; !ok || got != v {
				shared = false
				break
			}
		}
		if shared {
			out[k] = v
		}
	}
	return out
}
