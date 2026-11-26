package alert

import "testing"

func TestNameForGrafanaErrorAlerts(t *testing.T) {
	cases := []struct {
		labels KV
		want   string
	}{
		{KV{"alertname": "High CPU"}, "High CPU"},
		{KV{"alertname": "DatasourceError", "rulename": "Disk usage"}, "Disk usage (query failed)"},
		{KV{"alertname": "DatasourceNoData", "rulename": "Orders per hour"}, "Orders per hour (no data)"},
		{KV{"alertname": "DatasourceError"}, "DatasourceError"},
	}
	for _, c := range cases {
		if got := (Alert{Labels: c.labels}).Name(); got != c.want {
			t.Errorf("Name(%v) = %q, want %q", c.labels, got, c.want)
		}
	}
	n := Notification{Alerts: Alerts{{Labels: KV{"alertname": "DatasourceError", "rulename": "R"}}}}
	if n.Name() != "R (query failed)" {
		t.Errorf("notification name %q", n.Name())
	}
	mixed := Notification{CommonLabels: KV{"alertname": "X"}, Alerts: Alerts{{Labels: KV{"alertname": "A"}}, {Labels: KV{"alertname": "B"}}}}
	if mixed.Name() != "X" {
		t.Errorf("mixed name %q", mixed.Name())
	}
}

func TestLimitKeepsFiringFirstAndCounts(t *testing.T) {
	var as Alerts
	for i := 0; i < 5; i++ {
		as = append(as, Alert{Status: StatusResolved})
	}
	for i := 0; i < 5; i++ {
		as = append(as, Alert{Status: StatusFiring})
	}
	n := Notification{Alerts: as}.Limit(6)
	if len(n.Alerts) != 6 || len(n.Alerts.Firing()) != 5 || n.TruncatedAlerts != 4 {
		t.Fatalf("kept %d, firing %d, truncated %d", len(n.Alerts), len(n.Alerts.Firing()), n.TruncatedAlerts)
	}
	if n.FiringCount() != 5 || n.ResolvedCount() != 5 {
		t.Errorf("counts %d/%d", n.FiringCount(), n.ResolvedCount())
	}
	if same := (Notification{Alerts: as}).Limit(0); len(same.Alerts) != 10 {
		t.Error("Limit(0) should keep all")
	}
}
