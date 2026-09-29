//go:build ignore

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func sourceDiffDigest(ctx context.Context, root string) string {
	// Hash exact mounted source content, including uncommitted files, without retaining paths or diff text.
	h := sha256.New()
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Name() == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("source inventory contains non-regular file")
		}
		count++
		if count > 20000 {
			return errors.New("source inventory exceeds bound")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return errors.Join(err, errors.New("source inventory requires regular files"), f.Close())
		}
		normalized := filepath.ToSlash(rel)
		fmt.Fprintf(h, "%d:%s:%d\n", len(normalized), normalized, info.Size())
		n, readErr := io.Copy(h, inventoryReader{ctx: ctx, reader: io.LimitReader(f, 64<<20+1)})
		closeErr := f.Close()
		if n > 64<<20 {
			return errors.New("source file exceeds bound")
		}
		return errors.Join(readErr, closeErr)
	})
	if err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(h.Sum(nil))
}

type inventoryReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r inventoryReader) Read(body []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(body)
}

func (p *projection) snapshot(dir string) error {
	p.mu.Lock()
	body, err := json.Marshal(p.summary)
	p.mu.Unlock()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".live-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, writeErr := f.Write(body)
	if err = errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "live.json"))
}

func readSummary(dir string) (summary, error) {
	var s summary
	f, err := os.Open(filepath.Join(dir, "summary.json"))
	if errors.Is(err, os.ErrNotExist) {
		f, err = os.Open(filepath.Join(dir, "live.json"))
	}
	if err != nil {
		return s, err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, recordLimit+1))
	if err != nil {
		return s, err
	}
	if len(body) > recordLimit {
		return s, errors.New("summary exceeds bound")
	}
	err = json.Unmarshal(body, &s)
	return s, err
}

func metricText(s summary) string {
	var b strings.Builder
	// No dynamic labels: raw values, identities and unknown keys cannot add series.
	fmt.Fprintf(&b, "ardents_diagnostic_exit_code %d\nardents_diagnostic_timed_out %d\nardents_diagnostic_interrupted %d\nardents_diagnostic_capture_incomplete %d\nardents_diagnostic_source_tree_changed %d\nardents_diagnostic_samples_total %d\nardents_diagnostic_sample_failures_total %d\nardents_diagnostic_unrecognized_lines_total %d\nardents_diagnostic_invalid_lines_total %d\nardents_diagnostic_unknown_categories_total %d\nardents_diagnostic_oversized_lines_total %d\nardents_diagnostic_raw_dropped_bytes_total %d\nardents_diagnostic_event_dropped_bytes_total %d\nardents_diagnostic_sample_dropped_bytes_total %d\n", s.ExitCode, boolNumber(s.TimedOut), boolNumber(s.Interrupted), boolNumber(s.CaptureIncomplete), boolNumber(s.SourceChanged), s.Samples, s.SampleFailures, s.UnrecognizedLines, s.InvalidLines, s.UnknownCategories, s.OversizedLines, s.RawDroppedBytes, s.EventDroppedBytes, s.SampleDroppedBytes)
	for _, name := range resourceFields {
		if value, ok := s.Peak[name]; ok {
			fmt.Fprintf(&b, "ardents_diagnostic_peak_%s %s\n", name, strconv.FormatFloat(value, 'g', -1, 64))
		}
	}
	for _, kind := range strings.Fields("lifecycle resource resource-sample source-ready source-wave-accepted source-failed headless-runtime-failed headless-runtime-ready headless-runtime-permission-required headless-runtime-publication-refresh-failed headless-runtime-publication-withdrawal-failed headless-runtime-connection-operation-failed") {
		fmt.Fprintf(&b, "ardents_diagnostic_events_total{kind=%q} %d\n", kind, s.Counts[kind])
	}
	return b.String()
}
func boolNumber(value bool) int {
	if value {
		return 1
	}
	return 0
}

func handler(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
		io.WriteString(w, dashboard)
	})
	mux.HandleFunc("GET /summary", func(w http.ResponseWriter, r *http.Request) {
		s, err := readSummary(dir)
		if err != nil {
			http.Error(w, "capture unavailable", 503)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("GET /samples", func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(filepath.Join(dir, "samples.ndjson"))
		if err != nil {
			http.Error(w, "samples unavailable", 503)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/x-ndjson")
		io.Copy(w, io.LimitReader(f, recordLimit))
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		s, err := readSummary(dir)
		if err != nil {
			http.Error(w, "capture unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		io.WriteString(w, metricText(s))
	})
	return mux
}

func serve(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	dir := flags.String("dir", "", "one absolute evidence directory")
	addr := flags.String("listen", "127.0.0.1:8090", "local UI address; container bridge requires -container")
	container := flags.Bool("container", false, "allow container wildcard; publish host loopback only")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !filepath.IsAbs(*dir) || len(flags.Args()) != 0 {
		return errors.New("select absolute -dir")
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() && !(*container && host == "0.0.0.0") {
		return errors.New("UI must use loopback, or explicit container binding")
	}
	server := &http.Server{Addr: *addr, Handler: handler(*dir), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Printf("local diagnostics: http://%s; projected evidence only\n", *addr)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		end, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := server.Shutdown(end)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		result := <-done
		if errors.Is(result, http.ErrServerClosed) {
			result = nil
		}
		return errors.Join(err, result)
	}
}

// Embedded, dependency-free UI. Values use textContent, never HTML interpolation.
const dashboard = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Ardents local diagnostics</title>
<style>body{font:16px system-ui;background:#121b28;color:#e9f0f8;margin:2rem;max-width:1200px}h1{font-size:1.5rem}section{background:#1b293c;border-radius:12px;padding:1rem;margin:1rem 0}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:13px}table{width:100%;font-size:13px;text-align:left}td,th{padding:6px;border-bottom:1px solid #344457}.bad{color:#ffab91}.good{color:#9ce0bd}svg{width:100%;height:180px}a{color:#9bcbff}</style>
<h1>Ardents · local diagnostics</h1><p>Development observation. Readiness comes from the owner event; process existence is not readiness. Namespace network counters include all local fixture traffic.</p>
<section><h2 id="status">Waiting for capture</h2><pre id="health"></pre></section>
<section><h2>Process group RSS (MiB)</h2><svg id="rss" viewBox="0 0 1000 180" preserveAspectRatio="none"></svg><pre id="sample"></pre></section>
<section><h2>Latest owner transitions</h2><pre id="resource-state"></pre><table><thead><tr><th>Observed UTC</th><th>Kind</th><th>State</th><th>Carrier</th><th>Failure</th></tr></thead><tbody id="events"></tbody></table></section>
<section><h2>Resource peaks</h2><pre id="peaks"></pre><a href="/metrics">Bounded metrics</a></section>
<script>
async function update(){try{let r=await fetch('/summary',{cache:'no-store'});if(!r.ok)throw Error('Capture unavailable or not started');let s=await r.json();document.getElementById('resource-state').textContent=s.last_resource?JSON.stringify(s.last_resource,null,2):'No resource event observed';let done=s.finished&&!s.finished.startsWith('0001');let bad=s.capture_incomplete||s.timed_out||s.interrupted||(done&&s.exit_code!==0);let title=document.getElementById('status');title.textContent=done?(bad?'Failed / incomplete':'Command completed'):'Collecting';title.className=bad?'bad':'good';document.getElementById('health').textContent=JSON.stringify({started:s.started,finished:done?s.finished:null,exit_code:s.exit_code,timed_out:s.timed_out,interrupted:s.interrupted,capture_incomplete:s.capture_incomplete,samples:s.samples,sample_failures:s.sample_failures,invalid_lines:s.invalid_lines,unrecognized_lines:s.unrecognized_lines,oversized_lines:s.oversized_lines,raw_dropped_bytes:s.raw_dropped_bytes,event_dropped_bytes:s.event_dropped_bytes,sample_dropped_bytes:s.sample_dropped_bytes},null,2);let table=document.getElementById('events');table.replaceChildren();for(let e of (s.last_events||[]).slice(-32).reverse()){let tr=document.createElement('tr');for(let k of ['observed_at','kind','state','carrier','failure']){let td=document.createElement('td');td.textContent=e[k]||'—';tr.append(td)}table.append(tr)}document.getElementById('peaks').textContent=JSON.stringify(s.peak,null,2);let sr=await fetch('/samples',{cache:'no-store'});if(sr.ok){let text=await sr.text();let rows=text.trim().split('\n').filter(Boolean).map(x=>JSON.parse(x));let last=rows.at(-1);document.getElementById('sample').textContent=JSON.stringify(last||{samples:'not yet available'},null,2);let svg=document.getElementById('rss');svg.replaceChildren();if(rows.length){let maximum=Math.max(1,...rows.map(x=>x.rss_bytes));let path=document.createElementNS('http://www.w3.org/2000/svg','polyline');path.setAttribute('points',rows.map((x,i)=>[i*1000/Math.max(1,rows.length-1),170-160*x.rss_bytes/maximum].join(',')).join(' '));path.setAttribute('fill','none');path.setAttribute('stroke','#9bcbff');path.setAttribute('stroke-width','2');svg.append(path)}}}catch(e){document.getElementById('status').textContent=e.message;document.getElementById('status').className='bad'}}update();setInterval(update,2000);
</script></html>`

func snapshotCommand(args []string) error {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	socket := flags.String("socket", "", "private process debug Unix socket")
	out := flags.String("out", "", "new absolute output file outside source")
	kind := flags.String("kind", "runtime", "runtime, cpu, heap, allocs, goroutine, block, mutex, trace")
	seconds := flags.Int("seconds", 5, "CPU/trace duration, 1..30 seconds")
	sensitive := flags.Bool("sensitive", false, "explicitly permit secret-bearing profile evidence")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 || !filepath.IsAbs(*socket) || !filepath.IsAbs(*out) || *seconds < 1 || *seconds > 30 {
		return errors.New("select absolute socket/out and seconds 1..30")
	}
	route := "/runtime"
	switch *kind {
	case "runtime":
	case "cpu", "heap", "allocs", "goroutine", "block", "mutex":
		route = "/profile/" + *kind
	case "trace":
		route = "/trace"
	default:
		return errors.New("unknown capture kind")
	}
	if *kind != "runtime" && !*sensitive {
		return errors.New("profiles can contain secrets; -sensitive is required")
	}
	source, err := os.Getwd()
	if err != nil {
		return err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(*out))
	if err != nil {
		return err
	}
	path := filepath.Join(parent, filepath.Base(*out))
	if within(source, path) {
		return errors.New("profile evidence must be outside source")
	}
	if *kind == "cpu" || *kind == "trace" {
		route += "?seconds=" + strconv.Itoa(*seconds)
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", *socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("diagnostic redirect refused") }}
	response, err := client.Get("http://local-diagnostic" + route)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("diagnostic capture refused: HTTP %d", response.StatusCode)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(file, io.LimitReader(response.Body, 64<<20+1))
	closeErr := file.Close()
	if n > 64<<20 {
		copyErr = errors.Join(copyErr, errors.New("profile byte bound exceeded"))
	}
	if copyErr != nil || closeErr != nil {
		return errors.Join(copyErr, closeErr)
	}
	fmt.Printf("%s capture: %s (%d bytes)\n", *kind, path, n)
	return nil
}

// testTimings reports package and top-level test wall time separately: nested
// subtests overlap their parents and must not be added to the same total.
func testTimings(args []string) error {
	flags := flag.NewFlagSet("timings", flag.ContinueOnError)
	input := flags.String("input", "", "private go test -json output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	f, err := os.Open(*input)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 64<<20 {
		return errors.New("test timing input exceeds 64 MiB")
	}
	return writeTestTimings(f, os.Stdout)
}
func writeTestTimings(input io.Reader, output io.Writer) error {
	type timing struct {
		Package, Test, Action string
		Elapsed               float64
	}
	var packages, tests []timing
	bounded := &io.LimitedReader{R: input, N: (64 << 20) + 1}
	scanner := bufio.NewScanner(bounded)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	lines := 0
	for scanner.Scan() {
		lines++
		if lines > 500000 {
			return errors.New("test timing record bound exceeded")
		}
		var row timing
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return fmt.Errorf("invalid go test JSON at line %d: %w", lines, err)
		}
		if row.Action != "pass" && row.Action != "fail" && row.Action != "skip" {
			continue
		}
		if row.Test == "" {
			packages = append(packages, row)
		} else if !strings.Contains(row.Test, "/") {
			tests = append(tests, row)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, group := range []struct {
		name string
		rows []timing
	}{{"Packages", packages}, {"Top-level tests (subtests excluded)", tests}} {
		sort.Slice(group.rows, func(i, j int) bool { return group.rows[i].Elapsed > group.rows[j].Elapsed })
		if _, err := fmt.Fprintln(output, group.name); err != nil {
			return err
		}
		for i, row := range group.rows {
			if i == 20 {
				break
			}
			if _, err := fmt.Fprintf(output, "%.3fs %s %s %s\n", row.Elapsed, row.Action, row.Package, row.Test); err != nil {
				return err
			}
		}
	}
	if bounded.N <= 1 {
		return errors.New("test timing input exceeds 64 MiB")
	}
	if len(packages) == 0 {
		return errors.New("no terminal package result; timing capture is incomplete")
	}
	return nil
}
