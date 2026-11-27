package templates

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"text/template/parse"
	"time"
	"unicode/utf8"
)

// Limits for one render. Templates are code: an editor previewing a template,
// or an admin saving one, must not be able to hang or exhaust the plugin.
// Real alert templates stay far below these limits.
const (
	// MaxRenderTime is how long one template may run.
	MaxRenderTime = 5 * time.Second
	// MaxSteps caps loop iterations plus template calls in one render.
	MaxSteps = 1_000_000
	// MaxValueSize caps one value a function returns, such as a string or a
	// list, counted with everything it holds.
	MaxValueSize = 1 << 20
	// MaxRenderAllocation caps the total size of values functions return
	// during one render.
	MaxRenderAllocation = 64 << 20
	// maxCount caps count arguments, such as until 10000 or randAlpha 100.
	maxCount = 10_000
	// maxFormatWidth caps printf widths and precisions such as %5000s.
	maxFormatWidth = 10_000
)

// ErrLimit is returned when a template goes over a render limit.
var ErrLimit = errors.New("template went over a render limit")

// unsafeFuncs are Sprig functions templates cannot use: they read the
// plugin's environment, reach the network, or burn CPU on cryptography an
// email never needs.
var unsafeFuncs = []string{
	"env", "expandenv", "getHostByName",
	"bcrypt", "htpasswd", "derivePassword", "genPrivateKey", "buildCustomCert",
	"genCA", "genCAWithKey", "genSelfSignedCert", "genSelfSignedCertWithKey",
	"genSignedCert", "genSignedCertWithKey",
}

// guard enforces the limits of one render.
type guard struct {
	deadline time.Time
	steps    int
	alloc    int64
}

func newGuard() *guard { return &guard{deadline: time.Now().Add(MaxRenderTime)} }

func (g *guard) checkTime() error {
	if time.Now().After(g.deadline) {
		return fmt.Errorf("%w: it ran longer than %s", ErrLimit, MaxRenderTime)
	}
	return nil
}

// tick is called at the start of every loop iteration and template call.
func (g *guard) tick() (string, error) {
	g.steps++
	if g.steps > MaxSteps {
		return "", fmt.Errorf("%w: more than %d loop iterations and template calls", ErrLimit, MaxSteps)
	}
	return "", g.checkTime()
}

// funcs wraps every function in fm with the guard's checks.
func (g *guard) funcs(fm map[string]any) map[string]any {
	out := make(map[string]any, len(fm)+1)
	for name, fn := range fm {
		out[name] = g.wrap(name, fn)
	}
	out[tickFunc] = g.tick
	return out
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func (g *guard) wrap(name string, fn any) any {
	v := reflect.ValueOf(fn)
	t := v.Type()
	pre := preChecks[name]
	return reflect.MakeFunc(t, func(args []reflect.Value) []reflect.Value {
		// The template engine turns a panic in a function into a template
		// error, whatever the function's signature.
		if err := g.checkTime(); err != nil {
			panic(err)
		}
		if pre != nil {
			if err := pre(args); err != nil {
				panic(fmt.Errorf("%s: %w", name, err))
			}
		}
		var out []reflect.Value
		if t.IsVariadic() {
			out = v.CallSlice(args)
		} else {
			out = v.Call(args)
		}
		if len(out) > 0 && (len(out) != 1 || t.Out(0) != errorType) {
			if err := g.account(out[0]); err != nil {
				panic(fmt.Errorf("%s: %w", name, err))
			}
		}
		return out
	}).Interface()
}

func (g *guard) account(v reflect.Value) error {
	size := sizeOf(v, MaxValueSize+1)
	if size > MaxValueSize {
		return fmt.Errorf("%w: a value larger than %d bytes", ErrLimit, MaxValueSize)
	}
	g.alloc += int64(size)
	if g.alloc > MaxRenderAllocation {
		return fmt.Errorf("%w: functions produced more than %d bytes in total", ErrLimit, MaxRenderAllocation)
	}
	return nil
}

// sizeOf estimates the memory v holds, stopping once it passes limit.
func sizeOf(v reflect.Value, limit int) int {
	total := 0
	var walk func(v reflect.Value, depth int)
	walk = func(v reflect.Value, depth int) {
		if total > limit || !v.IsValid() || depth > 64 {
			return
		}
		switch v.Kind() {
		case reflect.String:
			total += v.Len()
		case reflect.Interface, reflect.Pointer:
			if !v.IsNil() {
				walk(v.Elem(), depth+1)
			}
		case reflect.Slice, reflect.Array:
			total += 16
			if v.Type().Elem().Kind() == reflect.Uint8 {
				total += v.Len()
				return
			}
			for i := 0; i < v.Len() && total <= limit; i++ {
				total += 8
				walk(v.Index(i), depth+1)
			}
		case reflect.Map:
			total += 48
			it := v.MapRange()
			for it.Next() && total <= limit {
				total += 16
				walk(it.Key(), depth+1)
				walk(it.Value(), depth+1)
			}
		case reflect.Struct:
			for i := 0; i < v.NumField() && total <= limit; i++ {
				walk(v.Field(i), depth+1)
			}
		default:
			total += 8
		}
	}
	walk(v, 0)
	return total
}

// Argument checks for functions whose result can be much larger than their
// arguments, run before the function allocates anything.
var preChecks = map[string]func(args []reflect.Value) error{
	"repeat": func(a []reflect.Value) error {
		return checkOutput(argInt(a, 0) * int64(argLen(a, 1)))
	},
	"until":        func(a []reflect.Value) error { return checkCount(abs(argInt(a, 0))) },
	"untilStep":    func(a []reflect.Value) error { return checkCount(stepCount(argInt(a, 0), argInt(a, 1), argInt(a, 2))) },
	"seq":          checkSeq,
	"indent":       checkIndent,
	"nindent":      checkIndent,
	"randAlpha":    func(a []reflect.Value) error { return checkCount(argInt(a, 0)) },
	"randAlphaNum": func(a []reflect.Value) error { return checkCount(argInt(a, 0)) },
	"randAscii":    func(a []reflect.Value) error { return checkCount(argInt(a, 0)) },
	"randNumeric":  func(a []reflect.Value) error { return checkCount(argInt(a, 0)) },
	"randBytes":    func(a []reflect.Value) error { return checkCount(argInt(a, 0)) },
	"wrapWith": func(a []reflect.Value) error {
		width := max(argInt(a, 0), 1)
		s := int64(argLen(a, 2))
		return checkOutput(s + (s/width+1)*int64(argLen(a, 1)))
	},
	"replace": func(a []reflect.Value) error {
		old, repl, src := argStr(a, 0), argStr(a, 1), argStr(a, 2)
		var n int
		if old == "" {
			n = utf8.RuneCountInString(src) + 1
		} else {
			n = strings.Count(src, old)
		}
		return checkOutput(int64(len(src)) + int64(n)*int64(len(repl)))
	},
	"regexReplaceAll":            checkRegexReplace(2, true),
	"mustRegexReplaceAll":        checkRegexReplace(2, true),
	"regexReplaceAllLiteral":     checkRegexReplace(2, false),
	"mustRegexReplaceAllLiteral": checkRegexReplace(2, false),
	"join": func(a []reflect.Value) error {
		if len(a) < 2 {
			return nil
		}
		l := a[1]
		for l.Kind() == reflect.Interface && !l.IsNil() {
			l = l.Elem()
		}
		if l.Kind() == reflect.Slice || l.Kind() == reflect.Array {
			return checkOutput(int64(l.Len()) * int64(argLen(a, 0)))
		}
		return nil
	},
	"printf": checkPrintf,
	"chunk": func(a []reflect.Value) error {
		if argInt(a, 0) < 1 {
			return errors.New("chunk size must be at least 1")
		}
		return nil
	},
}

func checkOutput(n int64) error {
	if n > MaxValueSize {
		return fmt.Errorf("%w: the result would be larger than %d bytes", ErrLimit, MaxValueSize)
	}
	return nil
}

func checkCount(n int64) error {
	if n > maxCount {
		return fmt.Errorf("%w: count %d is above %d", ErrLimit, n, maxCount)
	}
	return nil
}

func stepCount(start, stop, step int64) int64 {
	if step == 0 {
		return 0
	}
	return abs((stop-start)/step) + 1
}

func checkSeq(a []reflect.Value) error {
	if len(a) == 0 {
		return nil
	}
	params := a[0] // seq is variadic: one slice
	ints := make([]int64, 0, params.Len())
	for i := 0; i < params.Len(); i++ {
		ints = append(ints, toInt(params.Index(i)))
	}
	switch len(ints) {
	case 1:
		return checkCount(abs(ints[0]))
	case 2:
		return checkCount(abs(ints[1] - ints[0]))
	case 3:
		return checkCount(stepCount(ints[0], ints[2], ints[1]))
	}
	return nil
}

func checkIndent(a []reflect.Value) error {
	spaces := argInt(a, 0)
	if spaces > maxFormatWidth {
		return checkCount(spaces)
	}
	s := argStr(a, 1)
	return checkOutput(int64(len(s)) + spaces*int64(strings.Count(s, "\n")+2))
}

// checkRegexReplace bounds regexReplaceAll(regex, s, repl): at most len(s)+1
// matches, each replaced by repl, and each $ in repl can copy at most the
// whole match, whose lengths add up to len(s).
func checkRegexReplace(replArg int, expands bool) func(a []reflect.Value) error {
	return func(a []reflect.Value) error {
		s, repl := int64(argLen(a, 1)), argStr(a, replArg)
		size := s + (s+1)*int64(len(repl))
		if expands {
			size += int64(strings.Count(repl, "$")) * s
		}
		return checkOutput(size)
	}
}

var formatNumRe = regexp.MustCompile(`%[-+# 0]*(\d+|\*)?(?:\.(\d+|\*))?`)

// checkPrintf refuses huge widths and bounds the output by the format plus
// one copy of the largest argument per verb, since %[1]s can repeat an
// argument.
func checkPrintf(a []reflect.Value) error {
	format := argStr(a, 0)
	verbs, widths := 0, 0
	for _, m := range formatNumRe.FindAllStringSubmatch(format, -1) {
		verbs++
		for _, n := range m[1:] {
			switch {
			case n == "":
			case n == "*":
				widths += maxFormatWidth
			case len(n) > 5 || atoi(n) > maxFormatWidth:
				return fmt.Errorf("%w: width or precision above %d", ErrLimit, maxFormatWidth)
			default:
				widths += atoi(n)
			}
		}
	}
	largest := 0
	if len(a) > 1 {
		rest := a[1]
		for i := 0; i < rest.Len(); i++ {
			arg := rest.Index(i)
			if strings.Contains(format, "*") {
				if n := toInt(arg); n > maxFormatWidth {
					return fmt.Errorf("%w: width or precision above %d", ErrLimit, maxFormatWidth)
				}
			}
			largest = max(largest, sizeOf(arg, MaxValueSize+1))
		}
	}
	return checkOutput(int64(len(format)) + int64(widths) + int64(verbs)*int64(largest))
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func arg(a []reflect.Value, i int) reflect.Value {
	if i >= len(a) {
		return reflect.Value{}
	}
	v := a[i]
	for v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	return v
}

func argInt(a []reflect.Value, i int) int64 { return toInt(arg(a, i)) }

func toInt(v reflect.Value) int64 {
	for v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := v.Uint()
		if u > 1<<62 {
			return 1 << 62
		}
		return int64(u) // #nosec G115 -- bounded above
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if f > 1<<62 || f < -(1<<62) {
			return 1 << 62
		}
		return int64(f)
	case reflect.String:
		var n int64
		for _, c := range v.String() {
			if c < '0' || c > '9' || n > 1<<40 {
				break
			}
			n = n*10 + int64(c-'0')
		}
		return n
	}
	return 0
}

func argStr(a []reflect.Value, i int) string {
	v := arg(a, i)
	if v.Kind() == reflect.String {
		return v.String()
	}
	if v.IsValid() {
		return fmt.Sprint(v.Interface())
	}
	return ""
}

func argLen(a []reflect.Value, i int) int {
	v := arg(a, i)
	if v.Kind() == reflect.String {
		return v.Len()
	}
	return len(argStr(a, i))
}

// limitMessage shortens a template error caused by a limit. When the tick
// stopped the render, the engine's position points at the inserted tick, not
// at the template, so only the reason is kept.
func limitMessage(err error) error {
	if !errors.Is(err, ErrLimit) || !strings.Contains(err.Error(), tickFunc) {
		return err
	}
	msg := err.Error()
	if i := strings.Index(msg, ErrLimit.Error()); i >= 0 {
		return fmt.Errorf("%w%s", ErrLimit, msg[i+len(ErrLimit.Error()):])
	}
	return err
}

// tickFunc is the function the guard calls at each loop iteration and
// template call. Its name cannot clash with a function a template defines.
const tickFunc = "_beautifyAlertTick"

// tickSrc is parsed once per render to get a node that calls tickFunc and
// prints nothing: an assignment, so html/template does not escape it.
const tickSrc = "{{$__beautifyAlertTick := " + tickFunc + "}}"

// instrument inserts a tick at the start of every template body and range
// body, so loops and recursion count steps and see the deadline.
func instrument(trees []*parse.Tree, tick *parse.ActionNode) {
	var visit func(n parse.Node)
	visit = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				visit(c)
			}
		case *parse.RangeNode:
			visit(n.List)
			visit(n.ElseList)
			n.List.Nodes = append([]parse.Node{tick.Copy()}, n.List.Nodes...)
		case *parse.IfNode:
			visit(n.List)
			visit(n.ElseList)
		case *parse.WithNode:
			visit(n.List)
			visit(n.ElseList)
		}
	}
	for _, t := range trees {
		if t == nil || t.Root == nil {
			continue
		}
		visit(t.Root)
		t.Root.Nodes = append([]parse.Node{tick.Copy()}, t.Root.Nodes...)
	}
}

// checkPredefinedEscapers refuses html and urlquery anywhere except at the
// end of an output in an HTML template. html/template escapes output itself,
// and these two builtins cannot be wrapped there: inside loops they would
// grow strings without limit.
func checkPredefinedEscapers(trees []*parse.Tree) error {
	var err error
	var visit func(n parse.Node, lastOK bool)
	visitPipe := func(p *parse.PipeNode, lastOK bool) {
		if p == nil {
			return
		}
		for i, c := range p.Cmds {
			for j, a := range c.Args {
				if id, ok := a.(*parse.IdentifierNode); ok && (id.Ident == "html" || id.Ident == "urlquery") {
					atEnd := lastOK && i == len(p.Cmds)-1 && j == 0 && len(p.Decl) == 0
					if !atEnd && err == nil {
						err = fmt.Errorf("%q can only end an output, as in {{ .Value | %s }}; HTML templates escape output already", id.Ident, id.Ident)
					}
				}
				if sub, ok := a.(*parse.PipeNode); ok {
					visit(sub, false)
				}
			}
		}
	}
	visit = func(n parse.Node, lastOK bool) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				visit(c, true)
			}
		case *parse.ActionNode:
			visitPipe(n.Pipe, true)
		case *parse.PipeNode:
			visitPipe(n, lastOK)
		case *parse.RangeNode:
			visitPipe(n.Pipe, false)
			visit(n.List, true)
			visit(n.ElseList, true)
		case *parse.IfNode:
			visitPipe(n.Pipe, false)
			visit(n.List, true)
			visit(n.ElseList, true)
		case *parse.WithNode:
			visitPipe(n.Pipe, false)
			visit(n.List, true)
			visit(n.ElseList, true)
		case *parse.TemplateNode:
			visitPipe(n.Pipe, false)
		}
	}
	for _, t := range trees {
		if t != nil && t.Root != nil {
			visit(t.Root, true)
		}
	}
	return err
}
