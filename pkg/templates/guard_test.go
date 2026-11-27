package templates

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// Each attack must fail fast with ErrLimit (or a template error) and must not
// allocate much memory.
func TestRenderLimitsStopAttacks(t *testing.T) {
	attacks := map[string]config.Template{
		"integer range":           {HTML: `{{ range 3000000000 }}{{ end }}`},
		"nested ranges":           {HTML: `{{ range 100000 }}{{ range 100000 }}{{ end }}{{ end }}`},
		"until":                   {HTML: `{{ $x := until 50000000 }}{{ len $x }}`},
		"untilStep":               {HTML: `{{ $x := untilStep 0 900000000 1 }}`},
		"seq":                     {HTML: `{{ $x := seq 1 900000000 }}`},
		"repeat":                  {HTML: `{{ $x := repeat 400000000 "x" }}`},
		"indent":                  {HTML: `{{ $x := indent 400000000 "x" }}`},
		"randAlpha":               {HTML: `{{ $x := randAlpha 400000000 }}`},
		"replace with empty old":  {HTML: `{{ $s := repeat 100000 "x" }}{{ $x := replace "" $s $s }}`},
		"regexReplaceAll":         {HTML: `{{ $s := repeat 100000 "x" }}{{ $x := regexReplaceAll "" $s $s }}`},
		"printf width":            {HTML: `{{ printf "%900000000d" 1 }}`},
		"printf repeated arg":     {HTML: `{{ $s := repeat 100000 "x" }}{{ printf "%[1]s%[1]s%[1]s%[1]s%[1]s%[1]s%[1]s%[1]s%[1]s%[1]s%[1]s" $s }}`},
		"doubling with printf":    {HTML: `{{ $s := "xx" }}{{ range 60 }}{{ $s = printf "%s%s" $s $s }}{{ end }}`},
		"doubling with js":        {HTML: `{{ $s := "\\" }}{{ range 60 }}{{ $s = js $s }}{{ end }}`},
		"doubling with toJson":    {HTML: `{{ $s := "\"" }}{{ range 60 }}{{ $s = toJson $s }}{{ end }}`},
		"list of one big string":  {HTML: `{{ $s := repeat 500000 "x" }}{{ $l := list }}{{ range 10000 }}{{ $l = append $l $s }}{{ end }}{{ toJson $l }}`},
		"join with big separator": {HTML: `{{ $s := repeat 500000 "x" }}{{ join $s (until 10000) }}`},
		"exponential recursion":   {HTML: `{{ define "r" }}{{ if lt (len .) 40 }}{{ template "r" (append . 1) }}{{ template "r" (append . 1) }}{{ end }}{{ end }}{{ template "r" list }}`},
		"builtin-only recursion":  {HTML: `{{ define "r" }}{{ if ne . "&amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;amp;" }}{{ template "r" (html .) }}{{ template "r" (html .) }}{{ end }}{{ end }}{{ template "r" "&" }}`},
		"html in a loop":          {HTML: `{{ $s := "&" }}{{ range 1000000 }}{{ $s = html $s }}{{ end }}`},
		"RSA key":                 {HTML: `{{ genPrivateKey "rsa" }}`},
		"bcrypt":                  {HTML: `{{ bcrypt "x" }}`},
		"subject integer range":   {Subject: `{{ range 3000000000 }}{{ end }}`, HTML: "x"},
		"subject doubling":        {Subject: `{{ $s := "xx" }}{{ range 60 }}{{ $s = print $s $s }}{{ end }}`, HTML: "x"},
		"css inlining":            {HTML: `<style>` + strings.Repeat(".a{color:red}", 5000) + `</style>` + strings.Repeat(`<p class="a">x</p>`, 5000)},
	}
	for name, tmpl := range attacks {
		t.Run(name, func(t *testing.T) {
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			_, err := Render(tmpl, alert.Sample(time.Now()))
			took := time.Since(start)
			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			if err == nil {
				t.Fatal("rendered without error")
			}
			if !errors.Is(err, ErrLimit) && !strings.Contains(err.Error(), "can only end an output") && !strings.Contains(err.Error(), "not defined") {
				t.Errorf("unexpected error: %v", err)
			}
			if took > MaxRenderTime+time.Second {
				t.Errorf("took %s", took)
			}
			if grew := after.TotalAlloc - before.TotalAlloc; grew > 600<<20 {
				t.Errorf("allocated %d MB", grew>>20)
			}
			t.Logf("%s in %s: %v", name, took.Round(time.Millisecond), err)
		})
	}
}

// Normal templates must keep working under the limits.
func TestRenderLimitsAllowNormalTemplates(t *testing.T) {
	n := alert.Sample(time.Now())
	for _, tmpl := range []config.Template{
		{HTML: `{{ range .Alerts }}{{ range $k, $v := .Labels }}{{ $k }}={{ $v }} {{ end }}{{ end }}`},
		{HTML: `{{ printf "%s has %d alerts (%5.1f%%)" .Name .FiringCount 12.5 }}`},
		{HTML: `{{ $cols := list "a" "b" }}{{ range $cols }}<th>{{ . }}</th>{{ end }}{{ join ", " $cols }}`},
		{HTML: `{{ define "row" }}<tr><td>{{ .Name }}</td></tr>{{ end }}{{ range .Alerts }}{{ template "row" . }}{{ end }}`},
		{HTML: `{{ .Name | html }} {{ .Name | urlquery }} {{ js .Name }}`},
		{HTML: `{{ range until 500 }}{{ . }}{{ end }} {{ repeat 3 "ab" }} {{ indent 2 "x" }} {{ toJson .CommonLabels }}`},
		{Subject: `{{ .Name | html }} {{ printf "%d" .FiringCount }}`, HTML: "x"},
	} {
		if _, err := Render(tmpl, n); err != nil {
			t.Errorf("%q: %v", tmpl.HTML, err)
		}
	}
	for _, s := range Starters() {
		if _, err := Render(config.Template{Subject: s.Subject, HTML: s.HTML}, n); err != nil {
			t.Errorf("starter %s: %v", s.Name, err)
		}
	}
}

func TestHTMLInMiddleOfPipelineIsRefused(t *testing.T) {
	_, err := Render(config.Template{HTML: `{{ $x := html .Name }}`}, alert.Sample(time.Now()))
	if err == nil || !strings.Contains(err.Error(), "can only end an output") {
		t.Errorf("err = %v", err)
	}
}

func TestLimitErrorIsReadable(t *testing.T) {
	_, err := Render(config.Template{HTML: `{{ range 3000000000 }}{{ end }}`}, alert.Sample(time.Now()))
	if err == nil || strings.Contains(err.Error(), tickFunc) || !strings.Contains(err.Error(), "loop iterations") {
		t.Errorf("err = %v", err)
	}
}

func BenchmarkRenderDefaultStarter(b *testing.B) {
	n := alert.Sample(time.Now())
	tmpl := config.Template{Subject: Starters()[0].Subject, HTML: Starters()[0].HTML}
	for b.Loop() {
		if _, err := Render(tmpl, n); err != nil {
			b.Fatal(err)
		}
	}
}
