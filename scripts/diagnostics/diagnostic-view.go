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
	root, err := openEvidenceRoot(dir)
	if err != nil {
		return s, err
	}
	body, _, err := readEvidence(root, "summary.json", recordLimit)
	if errors.Is(err, os.ErrNotExist) {
		body, _, err = readEvidence(root, "live.json", recordLimit)
	}
	err = errors.Join(err, root.Close())
	if err != nil {
		return s, err
	}
	if !validJSONRecord(body) {
		return s, errors.New("summary invalid")
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return s, err
	}
	return safeSummaryForPanel(s)
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

func handler(dir string) http.Handler { return handlerWithComparison(dir, "") }

func handlerWithComparison(dir, compare string) http.Handler {
	return handlerWithConnection(dir, compare, "")
}

func handlerWithConnection(dir, compare, socket string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'")
		io.WriteString(w, dashboard)
	})
	mux.HandleFunc("GET /connection", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if socket == "" {
			json.NewEncoder(w).Encode(connectionObservation{State: "unselected", Records: []connectionObservationRecord{}, Missing: []string{}})
			return
		}
		observation, err := readConnectionObservation(r.Context(), socket)
		if err != nil {
			http.Error(w, "connection observations unavailable", http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(observation)
	})
	mux.HandleFunc("GET /report", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildAssistantReport(dir, compare, time.Now().UTC()))
	})
	mux.HandleFunc("GET /summary", func(w http.ResponseWriter, r *http.Request) {
		s, err := readSummary(dir)
		if err != nil {
			http.Error(w, "capture unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("GET /samples", func(w http.ResponseWriter, r *http.Request) {
		samples, err := safeSamplesForPanel(dir)
		if err != nil {
			http.Error(w, "samples unavailable or incomplete", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/x-ndjson")
		encoder := json.NewEncoder(w)
		for _, row := range samples {
			if err := encoder.Encode(row); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		s, err := readSummary(dir)
		if err != nil {
			http.Error(w, "capture unavailable", http.StatusServiceUnavailable)
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
	compare := flags.String("compare", "", "optional second absolute evidence directory")
	socket := flags.String("connection-socket", "", "optional owner-private live process socket; not associated with stored run")
	addr := flags.String("listen", "127.0.0.1:8090", "local UI address; container bridge requires -container")
	container := flags.Bool("container", false, "allow container wildcard; publish host loopback only")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *socket != "" && (!filepath.IsAbs(*socket) || filepath.Clean(*socket) != *socket) {
		return errors.New("select canonical absolute connection-socket")
	}
	if !filepath.IsAbs(*dir) || *compare != "" && !filepath.IsAbs(*compare) || len(flags.Args()) != 0 {
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
	server := &http.Server{Addr: *addr, Handler: handlerWithConnection(*dir, *compare, *socket), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
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

// Embedded, dependency-free UI. Evidence values use textContent, never HTML.
// Export is a frozen, previewed browser-local projection, never a control route.
const dashboard = `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Ardents · Диагностика</title>
<style>
:root{color-scheme:dark;--bg:#0b111b;--panel:#131e2d;--border:#29384b;--text:#e8eef8;--muted:#a1b0c5;--accent:#83b9ff;--bad:#ff9c9c;--good:#87dfb6;--warn:#f0cc82}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.55 system-ui,sans-serif}main{max-width:1440px;margin:auto;padding:28px 32px 48px}header{display:flex;justify-content:space-between;align-items:center;gap:20px;margin-bottom:24px}.brand{letter-spacing:.14em;font-size:12px;color:var(--accent);font-weight:700}h1{font-size:28px;margin:3px 0}h2{font-size:17px;margin:0 0 14px}h3{font-size:14px;margin:0 0 8px}p{margin:8px 0}.muted,small{color:var(--muted)}.toolbar,.tabs,.tags{display:flex;gap:8px;flex-wrap:wrap;align-items:center}button,select{font:inherit;background:#1d2c40;color:var(--text);border:1px solid #39516e;border-radius:8px;padding:8px 12px;cursor:pointer}button:hover{background:#29405c}button:disabled{opacity:.45;cursor:default}button:focus-visible,a:focus-visible,summary:focus-visible,select:focus-visible,input:focus-visible{outline:2px solid var(--accent);outline-offset:3px}.primary{background:#315d94;border-color:#5288cb}.tabs{margin:0 0 22px}.tabs a{color:var(--muted);text-decoration:none;padding:7px 12px;background:var(--panel);border:1px solid var(--border);border-radius:8px}a{color:var(--accent)}.grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:18px}.card{background:var(--panel);border:1px solid var(--border);border-radius:14px;padding:22px;min-width:0}.full{grid-column:span 12}.half{grid-column:span 6}.third{grid-column:span 4}.hero{border-left:4px solid var(--accent)}.hero.bad{border-left-color:var(--bad)}.hero.good{border-left-color:var(--good)}.hero.warn{border-left-color:var(--warn)}.hero.bad,.hero.good,.hero.warn{color:var(--text)}.hero.bad .status-title{color:var(--bad)}.hero.good .status-title{color:var(--good)}.hero.warn .status-title{color:var(--warn)}.status-title{font-size:24px;margin:2px 0 8px}.badge{font-size:12px;border-radius:6px;padding:4px 8px;background:#223147;display:inline-block}.bad{color:var(--bad)}.good{color:var(--good)}.warn{color:var(--warn)}.stat{font-size:25px;letter-spacing:-.02em;font-weight:650;margin:6px 0}.stat-label{font-size:12px;color:var(--muted)}.metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-top:18px}.metric{padding:14px;background:#0e1724;border-radius:9px;border:1px solid var(--border)}.facts{display:grid;grid-template-columns:minmax(140px,1fr) minmax(0,2fr);gap:6px 14px;font-size:13px}.facts dt{color:var(--muted)}.facts dd{margin:0;overflow-wrap:anywhere;font-family:ui-monospace,monospace}.list{display:grid;gap:10px}.notice{padding:12px 14px;background:#0e1724;border:1px solid var(--border);border-radius:8px}.check{border:1px solid var(--border);border-radius:10px;padding:16px}.check p{color:var(--muted)}.check pre{background:#0b1421;padding:12px;border-radius:8px}.check button{margin-top:8px}.section-head{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:14px}.section-head h2{margin:0}details{margin-top:14px}summary{cursor:pointer;color:var(--accent)}pre{white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.6 ui-monospace,monospace;margin:10px 0 0;max-height:400px;overflow:auto}table{width:100%;border-collapse:collapse;text-align:left;font-size:12px}th{color:var(--muted);font-weight:500}th,td{padding:10px 8px;border-bottom:1px solid var(--border);overflow-wrap:anywhere}.table-scroll{overflow:auto}svg{display:block;width:100%;height:190px}.axis{display:flex;justify-content:space-between;color:var(--muted);font-size:11px}.empty{color:var(--muted);padding:16px 0}[hidden]{display:none!important}.export-preview{max-height:450px;padding:16px;background:#0b1421;border:1px solid var(--border);border-radius:8px}.status-note{min-height:22px}footer{margin-top:24px;color:var(--muted);font-size:12px}@media(max-width:1000px){.third{grid-column:span 6}.third:last-child{grid-column:span 12}}@media(max-width:700px){main{padding:18px 14px}header{align-items:flex-start;flex-direction:column}.half,.third{grid-column:span 12}.card{padding:17px}.metrics{grid-template-columns:1fr}.facts{grid-template-columns:1fr}h1{font-size:24px}.stat{font-size:22px}}
</style>
<main><header><div><div class="brand">ARDENTS / LOCAL OBSERVATIONS</div><h1>Помощник диагностики</h1><div class="muted">Сохранённый прогон · факты, пробелы и следующий шаг</div></div><div class="toolbar"><span id="refresh-state" class="muted" role="status">Загрузка…</span><button id="pause" type="button">Приостановить обновление</button><button id="refresh" type="button">Обновить</button></div></header>
<nav class="tabs" aria-label="Разделы диагностики"><a href="#overview">Обзор</a><a href="#connection">Reader Connection</a><a href="#checks">Следующие проверки</a><a href="#resources">Ресурсы</a><a href="#observations">Наблюдения</a><a href="#comparison">Сравнение</a><a href="#export">Выгрузка</a></nav>
<div class="grid">
<section id="overview" class="card full hero"><div class="tags"><span class="badge">Результат команды</span><span id="capture" class="badge">Сбор неизвестен</span><span id="exit" class="badge">Exit —</span></div><h2 id="status" class="status-title" aria-live="polite">Ожидаем данные</h2><p id="verdict"></p><small>Это локальные инженерные наблюдения. Успех команды и исторический READY не доказывают текущую готовность сети.</small><div class="metrics"><div class="metric"><div class="stat-label">Время supervisor</div><div id="duration" class="stat">—</div><small>Включает подготовку и сбор</small></div><div class="metric"><div class="stat-label">Максимум RSS группы</div><div id="rss-peak" class="stat">—</div><small>Наблюдаемый максимум</small></div><div class="metric"><div class="stat-label">Сохранено samples</div><div id="sample-count" class="stat">—</div><small id="losses">Потери неизвестны</small></div></div></section>
<section id="connection" class="card full"><div class="section-head"><h2>Одна Reader Connection</h2><span id="connection-state" class="badge" role="status">Источник не выбран</span></div><p class="muted">Живой снимок выбранного приватного процесса. Его связь с сохранённым прогоном не установлена. Первая операция в окне 10 минут, до 64 записей; без Targets, ключей и raw ошибок.</p><div id="connection-outcome" class="notice">Для подключения укажите connection-socket при запуске панели.</div><div class="table-scroll"><table><thead><tr><th>Этап</th><th>Наблюдение</th><th>От начала</th><th>Длительность</th><th>Остаток срока Context на входе</th></tr></thead><tbody id="connection-records"></tbody></table></div><p id="connection-gaps" class="muted"></p><small>Application EOF и аутентификация не означают joined cleanup. Отсутствующий этап означает отсутствие наблюдения; причину и готовность сети это не доказывает. Снимок Connection не включается в выгрузку отчёта прогона.</small></section>
<section class="card half"><h2>Первый наблюдаемый отказ</h2><div id="first-failure"></div><details><summary>Остальные отказы и cleanup</summary><div id="failures" class="list"></div></details></section>
<section class="card half"><h2>Пробелы наблюдения</h2><div id="gaps" class="list"></div></section>
<section id="checks" class="card full"><div class="section-head"><h2>Следующие проверки</h2><span class="muted">Шаблоны запускаются вручную</span></div><div id="next-checks" class="list"></div><div id="copy-status" class="status-note muted" role="status"></div></section>
<section id="resources" class="card half"><h2>RSS группы процессов</h2><div class="axis"><span id="rss-scale">—</span><span>Наблюдения, не оценка нагрузки</span></div><svg id="rss" viewBox="0 0 1000 190" preserveAspectRatio="none" role="img" aria-label="RSS группы процессов во времени"></svg><div class="axis"><span id="time-start">—</span><span id="time-end">—</span></div><details><summary>Последний sample</summary><pre id="sample"></pre></details></section>
<section class="card half"><h2>Контейнер: ресурсы и давление</h2><dl id="resource-facts" class="facts"></dl><h3>PSI avg10 · наблюдаемые пики</h3><div id="pressure" class="tags"></div><p class="muted">Счётчики cgroup и сетевого namespace общие для контейнера. Они не определяют виновный процесс.</p><details><summary>Последнее resource event и peaks</summary><pre id="resource-state"></pre><pre id="peaks"></pre></details></section>
<section id="observations" class="card full"><div class="section-head"><h2>Последние переходы владельцев</h2><label>Показать <select id="event-filter"><option value="all">Все</option><option value="failure">Отказы</option><option value="lifecycle">Lifecycle</option><option value="resource">Ресурсы</option></select></label></div><p id="event-empty" class="empty">Наблюдения ещё не получены</p><div class="table-scroll"><table><thead><tr><th>Collector UTC</th><th>Категория</th><th>State</th><th>Carrier</th><th>Отказ</th></tr></thead><tbody id="events"></tbody></table></div><small>До 32 последних переходов. Фильтр действует только на этот tail; порядок collector не доказывает причинность.</small></section>
<section id="comparison" class="card full"><h2>Сравнение выбранных прогонов</h2><div id="comparison-content"></div></section>
<section class="card full"><h2>Условия и целостность сбора</h2><dl id="conditions" class="facts"></dl><details><summary>Счётчики collector</summary><pre id="health"></pre></details><a href="/metrics">Метрики collector</a></section>
<section id="export" class="card full"><h2>Выгрузка безопасного отчёта</h2><p class="muted">Выгружаются итоги, ресурсы, категории отказов, пробелы и рекомендации. Raw logs, пути, аргументы команд, profiles и история peers не включаются. Даже такой отчёт содержит локальные метаданные: проверьте предпросмотр перед передачей.</p><label><input id="export-provenance" type="checkbox"> Включить source/image digests и версии инструментов</label><div class="toolbar" style="margin-top:12px"><button id="preview-export" type="button" disabled>Предпросмотр отчёта</button><button id="download-export" class="primary" type="button" disabled>Скачать просмотренный JSON</button><button id="copy-export" type="button" disabled>Копировать просмотренный JSON</button></div><p id="export-status" class="muted" role="status">Сначала получите данные и откройте предпросмотр.</p><pre id="export-preview" class="export-preview" hidden></pre></section>
</div><footer>Приватный локальный инструмент · без автоматического выполнения рекомендаций и изменения State</footer></main>
<script>
'use strict';
const $=id=>document.getElementById(id);
let latest=null,latestEvents=[],paused=false,busy=false,exportText=null,renderedReport=null;
const statusNames={'command-completed':'Команда завершилась успешно','command-failed':'Команда завершилась с ошибкой','interrupted':'Прогон прерван','timed-out':'Лимит времени исчерпан','collecting':'Сбор продолжается','observation-stale':'Наблюдения устарели','command-completed-with-incomplete-evidence':'Команда завершилась, сбор неполон','unavailable':'Данные недоступны'};
const text=(id,value)=>{$(id).textContent=value};
function element(tag,value,className){let n=document.createElement(tag);if(value!==undefined)n.textContent=value;if(className)n.className=className;return n}
function bytes(value){if(value===undefined)return 'нет наблюдения';let units=['B','KiB','MiB','GiB'];let i=0;while(value>=1024&&i<3){value/=1024;i++}return value.toFixed(i?1:0)+' '+units[i]}
function duration(value){return Number.isFinite(value)?value.toFixed(1)+' с':'—'}
function facts(id,entries){let root=$(id);root.replaceChildren();for(let [label,value] of entries){root.append(element('dt',label),element('dd',String(value)))}if(!entries.length)root.append(element('dt','Нет наблюдений'))}
function notice(root,value,className){root.append(element('div',value,'notice '+(className||'')))}
function describeFailure(f){return f.owner+' / '+f.kind+' / '+(f.category||f.state||'—')+' · запись #'+f.record}
function renderRun(a){let r=a.run;let bad=!r.complete||r.timed_out||r.interrupted||(r.finished&&r.exit_code!==0)||(!r.finished&&r.status!=='collecting');$('overview').className='card full hero '+(bad?'bad':r.finished?'good':'warn');text('status',statusNames[r.status]||r.status);text('capture',r.complete?'Сбор полный':'Сбор неполон');text('exit',r.finished?'Exit '+r.exit_code:'Exit ещё не получен');text('verdict',r.status+' · '+(r.source_changed?'Исходники изменились; приёмка недействительна.':'Результат команды отделён от полноты наблюдений.'));text('duration',duration(r.duration_seconds));text('rss-peak',bytes(r.resource_facts['process-group-rss-peak']));let first=$('first-failure');first.replaceChildren();if(r.first_observed_failure){notice(first,describeFailure(r.first_observed_failure),'bad');first.append(element('p','Первопричина этим наблюдением не доказана.','muted'))}else notice(first,'Явный отказ владельца не найден в безопасной истории. Общий exit code не называет причину.','muted');let failures=$('failures');failures.replaceChildren();for(let f of r.failures||[])notice(failures,describeFailure(f));if(!failures.childNodes.length)notice(failures,'Нет сохранённых категорий отказа.');let gaps=$('gaps');gaps.replaceChildren();for(let g of r.gaps||[])notice(gaps,r.gap_descriptions[g]||g);if(!gaps.childNodes.length)notice(gaps,'Заявленные пробелы не обнаружены.');let checks=$('next-checks');checks.replaceChildren();for(let c of r.next_checks||[]){let card=element('article',undefined,'check');card.append(element('h3',c.id),element('p',c.reason));let dl=element('dl',undefined,'facts');for(let [label,value] of [['Условия',c.prerequisites],['Лимит',c.budget],['Чувствительные данные',c.sensitive?'Да':'Нет'],['Ожидается',c.expected]])dl.append(element('dt',label),element('dd',value));let command=element('pre',c.invocation);let copy=element('button','Копировать шаблон');copy.type='button';copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(c.invocation);text('copy-status','Шаблон скопирован. Замените placeholders перед ручным запуском.')}catch{let selection=getSelection();let range=document.createRange();range.selectNodeContents(command);selection.removeAllRanges();selection.addRange(range);text('copy-status','Шаблон выделен: используйте обычное копирование.')}});card.append(dl,command,copy);checks.append(card)}if(!checks.childNodes.length)notice(checks,'Следующая проверка не предложена.');facts('resource-facts',Object.entries(r.resource_facts||{}).map(([key,v])=>[key,/memory|rss/.test(key)&&!key.startsWith('memory.events')?bytes(v):v]));let pressure=$('pressure');pressure.replaceChildren();for(let [key,v] of Object.entries(r.container_pressure_peaks||{}))pressure.append(element('span',key+' '+v.toFixed(2)+' %','badge'));if(!pressure.childNodes.length)pressure.append(element('span','Нет наблюдений','muted'));facts('conditions',Object.entries(r.conditions||{}).map(([key,v])=>[key,typeof v==='object'?JSON.stringify(v):v]));renderComparison(a.comparison);$('preview-export').disabled=false}
function renderComparison(c){let root=$('comparison-content');root.replaceChildren();if(!c){notice(root,'Второй прогон не выбран. Его директория задаётся при запуске serve.');return}notice(root,'Условия: '+c.conditions+' · '+((c.differences||[]).join(', ')||'известных различий нет'));notice(root,'Другой прогон: '+(statusNames[c.other.status]||c.other.status)+' · exit '+c.other.exit_code+' · сбор '+(c.other.complete?'полный':'неполный'),c.other.exit_code!==0||!c.other.complete?'bad':'');notice(root,'Разница времени supervisor: '+duration(c.duration_difference_seconds)+'. Ускорение не установлено.');let table=element('table');let head=element('tr');for(let label of ['Ресурс','Другой прогон','Текущий прогон'])head.append(element('th',label));table.append(head);for(let [key,v] of Object.entries(c.resource_changes||{})){let row=element('tr');for(let value of [key,v.before,v.after])row.append(element('td',value));table.append(row)}root.append(table);let details=element('details');details.append(element('summary','Пробелы другого прогона'));for(let g of c.other.gaps||[])notice(details,c.other.gap_descriptions[g]||g);root.append(details)}
function renderEvents(){let filter=$('event-filter').value;let rows=latestEvents.filter(e=>filter==='all'||filter==='failure'&&(e.failure||e.state==='FAILED'||/failed|permission-required/.test(e.kind))||filter==='lifecycle'&&e.kind==='lifecycle'||filter==='resource'&&/resource/.test(e.kind));let table=$('events');table.replaceChildren();for(let e of rows){let row=element('tr');for(let key of ['observed_at','kind','state','carrier','failure'])row.append(element('td',e[key]||'—'));table.append(row)}text('event-empty',rows.length?'':'В выбранном tail нет таких наблюдений');$('event-empty').hidden=rows.length>0}
function renderSummary(s){text('sample-count',s.samples);text('losses','Потери: raw '+s.raw_dropped_bytes+' B · events '+s.event_dropped_bytes+' B · samples '+s.sample_dropped_bytes+' B');text('health',JSON.stringify(s,null,2));text('resource-state',s.last_resource?JSON.stringify(s.last_resource,null,2):'Resource event не получен');text('peaks',JSON.stringify(s.peak||{},null,2));latestEvents=(s.last_events||[]).slice(-32).reverse();renderEvents()}
function renderSamples(rows){let svg=$('rss');svg.replaceChildren();let last=rows.at(-1);text('sample',JSON.stringify(last||{samples:'нет наблюдений'},null,2));if(!rows.length){text('rss-scale','Нет samples');text('time-start','—');text('time-end','—');return}let max=1;for(let r of rows)max=Math.max(max,r.rss_bytes);text('rss-scale','Шкала: 0 — '+bytes(max));text('time-start',rows[0].at);text('time-end',last.at);let points=[];let step=Math.max(1,Math.ceil(rows.length/1000));let first=Date.parse(rows[0].at),end=Date.parse(last.at);for(let i=0;i<rows.length;i+=step){let peak=rows[i];for(let j=i+1;j<Math.min(i+step,rows.length);j++)if(rows[j].rss_bytes>peak.rss_bytes)peak=rows[j];let x=end>first?(Date.parse(peak.at)-first)*1000/(end-first):500;points.push([x,180-170*peak.rss_bytes/max].join(','))}let path=document.createElementNS('http://www.w3.org/2000/svg','polyline');path.setAttribute('points',points.join(' '));path.setAttribute('fill','none');path.setAttribute('stroke','#83b9ff');path.setAttribute('stroke-width','2');svg.append(path)}
// Select fields explicitly. The optional provenance is visible before download.
function exportRun(r,provenance){let result={status:r.status,complete:r.complete,finished:r.finished,exit_code:r.exit_code,timed_out:r.timed_out,interrupted:r.interrupted,source_changed:r.source_changed,duration_seconds:r.duration_seconds,resource_facts:r.resource_facts,container_pressure_peaks:r.container_pressure_peaks,failures:(r.failures||[]).map(f=>({owner:f.owner,kind:f.kind,state:f.state,category:f.category,record:f.record})),gaps:r.gaps,gap_descriptions:r.gap_descriptions,next_checks:r.next_checks};if(r.first_observed_failure){let f=r.first_observed_failure;result.first_observed_failure={owner:f.owner,kind:f.kind,state:f.state,category:f.category,record:f.record}}if(provenance)result.conditions=r.conditions;return result}
function invalidateExport(){exportText=null;$('export-preview').hidden=true;$('download-export').disabled=true;$('copy-export').disabled=true;text('export-status','Настройки изменены. Откройте новый предпросмотр.');text('export-preview','')}
$('preview-export').addEventListener('click',()=>{if(!latest)return;let provenance=$('export-provenance').checked;let result={scope:'Private local engineering observations; no readiness, qualification or causal-root claim',run:exportRun(latest.run,provenance)};if(latest.comparison){let c=latest.comparison;result.comparison={other:exportRun(c.other,provenance),conditions:c.conditions,differences:c.differences,resource_changes:c.resource_changes,duration_difference_seconds:c.duration_difference_seconds}}let candidate=JSON.stringify(result,null,2);if(new TextEncoder().encode(candidate).length>262144){invalidateExport();text('export-status','Проекция превышает лимит 256 KiB; выгрузка недоступна.');return}exportText=candidate;text('export-preview',exportText);$('export-preview').hidden=false;$('download-export').disabled=false;$('copy-export').disabled=false;text('export-status','Предпросмотр зафиксирован. Обновления панели его не меняют; скачать можно только этот JSON.')});
$('export-provenance').addEventListener('change',invalidateExport);
$('download-export').addEventListener('click',()=>{if(exportText===null)return;let url=URL.createObjectURL(new Blob([exportText],{type:'application/json'}));let link=element('a');link.href=url;link.download='ardents-diagnostic-report.json';document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);text('export-status','Скачивание просмотренного снимка запрошено. Если браузер его не сохраняет, используйте копирование JSON.')});
$('copy-export').addEventListener('click',async()=>{if(exportText===null)return;try{await navigator.clipboard.writeText(exportText);text('export-status','Просмотренный JSON скопирован. Автоматической передачи отчёта нет.')}catch{let selection=getSelection();let range=document.createRange();range.selectNodeContents($('export-preview'));selection.removeAllRanges();selection.addRange(range);text('export-status','Просмотренный JSON выделен: используйте обычное копирование.')}});
$('event-filter').addEventListener('change',renderEvents);
$('pause').addEventListener('click',()=>{paused=!paused;text('pause',paused?'Продолжить обновление':'Приостановить обновление');text('refresh-state',paused?'Обновление приостановлено':'Обновление включено');if(!paused)update()});$('refresh').addEventListener('click',()=>update(true));
const connectionStages={'admission':'Admission','activation':'Активация сессии','worker-launch':'Запуск worker','worker-activation':'Ограниченная activation worker','worker-operation':'Получение worker operation','introduction':'Подготовка Introduction','service-join':'JOIN с аутентификацией','service-authentication':'TLS и Service authentication','local-request':'Локальный запрос','document-exchange':'Обмен документом','service-close':'Закрытие Service','worker-close':'Закрытие worker','current-owner':'Проверка текущего владельца','application-response':'Локальный ответ','caller-join':'Join отмены вызывающего','session-release':'Освобождение сессии'};
function renderConnection(a){const states={'unselected':'Источник не выбран','waiting':'Ожидаем первую Reader операцию','active':'Операция выполняется','joined':'Cleanup завершён и joined','expired':'Окно захвата истекло','incomplete':'Наблюдения неполны'};text('connection-state',states[a.state]||'Наблюдения недоступны');text('connection-outcome',a.state==='unselected'?'Для подключения укажите connection-socket при запуске панели.':'Исход: '+({completed:'завершено',failed:'отказ',canceled:'отмена',deadline:'истёк срок'}[a.outcome]||'ещё не наблюдён')+' · потеряно записей: '+a.lost_records);let root=$('connection-records');root.replaceChildren();for(let r of a.records||[]){let row=element('tr');let state={started:'начало',completed:'завершён',failed:'отказ',canceled:'отмена',deadline:'истёк срок'}[r.state]||'неизвестно';if(r.context_stop)state+=' · Context: '+({canceled:'отменён',deadline:'срок истёк'}[r.context_stop]||'неизвестно');let values=[connectionStages[r.stage]||'Неизвестный этап',state,(r.elapsed_ns/1e6).toFixed(3)+' мс',r.duration_ns===undefined?'—':(r.duration_ns/1e6).toFixed(3)+' мс',r.state==='started'?(r.remaining_budget_ns===undefined?'не наблюдён':(r.remaining_budget_ns/1e6).toFixed(3)+' мс'):'—'];for(let v of values)row.append(element('td',v));root.append(row)}text('connection-gaps',a.state==='unselected'?'':('Без наблюдения завершения: '+((a.missing_stages||[]).map(s=>connectionStages[s]||s).join(', ')||'нет')))}
async function updateConnection(){try{let response=await fetch('/connection',{cache:'no-store'});if(!response.ok)throw Error('unavailable');renderConnection(await response.json())}catch{renderConnection({state:'unavailable',records:[],missing_stages:[]});text('connection-outcome','Свежий снимок не получен. Исход и завершение неизвестны.');text('connection-gaps','Проверьте выбранный приватный процесс и доступ к его socket.')}}

async function update(manual=false){if(busy||paused&&!manual)return;busy=true;try{let reportResponse=await fetch('/report',{cache:'no-store'});if(!reportResponse.ok)throw Error('Отчёт недоступен');let a=await reportResponse.json();latest=a;let key=JSON.stringify(a);if(key!==renderedReport){renderRun(a);renderedReport=key}$('preview-export').disabled=false;let responses=await Promise.all([fetch('/summary',{cache:'no-store'}),fetch('/samples',{cache:'no-store'})]);if(responses[0].ok)renderSummary(await responses[0].json());else{latestEvents=[];renderEvents();text('sample-count','—');text('losses','Summary недоступен');text('health','Summary недоступен');text('resource-state','Resource event недоступен');text('peaks','—')}if(responses[1].ok){let body=await responses[1].text();renderSamples(body.trim().split('\n').filter(Boolean).map(x=>JSON.parse(x)))}else renderSamples([]);text('refresh-state',(paused?'Пауза · ':'Обновлено · ')+new Date().toLocaleTimeString('ru-RU'))}catch{latest=null;renderedReport=null;$('preview-export').disabled=true;text('capture','Текущий сбор неизвестен');text('exit','Текущий exit неизвестен');if(exportText!==null)text('export-status','Свежие данные недоступны. Просмотренный снимок сохранён; скачать можно прежний JSON.');text('status','Наблюдения недоступны');$('overview').className='card full hero bad';text('verdict','Не удалось получить полный ответ. Предыдущие показатели могут быть устаревшими.');text('refresh-state','Ошибка обновления')}finally{await updateConnection();busy=false}}
update();setInterval(()=>update(),2000);
</script></html>`

func snapshotCommand(args []string) error {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	socket := flags.String("socket", "", "private process debug Unix socket")
	out := flags.String("out", "", "new absolute output file outside source")
	kind := flags.String("kind", "runtime", "runtime, connection, cpu, heap, allocs, goroutine, block, mutex, trace")
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
	case "connection":
		route = "/connection"
	case "cpu", "heap", "allocs", "goroutine", "block", "mutex":
		route = "/profile/" + *kind
	case "trace":
		route = "/trace"
	default:
		return errors.New("unknown capture kind")
	}
	if *kind != "runtime" && *kind != "connection" && !*sensitive {
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
	if *kind == "connection" {
		body, err := io.ReadAll(io.LimitReader(response.Body, 32<<10+1))
		if err != nil {
			return errors.New("connection snapshot read failed")
		}
		observation, err := decodeConnectionObservation(body)
		if err != nil {
			return err
		}
		return writeJSON(path, observation)
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

func readConnectionObservation(ctx context.Context, socket string) (connectionObservation, error) {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("diagnostic redirect refused") }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://local-diagnostic/connection", nil)
	if err != nil {
		return connectionObservation{}, errors.New("connection request unavailable")
	}
	response, err := client.Do(request)
	if err != nil {
		return connectionObservation{}, errors.New("connection socket unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return connectionObservation{}, errors.New("connection capture refused")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<10+1))
	if err != nil {
		return connectionObservation{}, errors.New("connection capture read failed")
	}
	return decodeConnectionObservation(body)
}

func connectionCommand(args []string) error {
	flags := flag.NewFlagSet("connection", flag.ContinueOnError)
	socket := flags.String("socket", "", "explicit absolute owner-private process socket")
	jsonOutput := flags.Bool("json", false, "print fixed safe snapshot JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !filepath.IsAbs(*socket) || filepath.Clean(*socket) != *socket || len(flags.Args()) != 0 {
		return errors.New("select canonical absolute socket")
	}
	observation, err := readConnectionObservation(context.Background(), *socket)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(observation)
	}
	fmt.Printf("Reader capture: %s; outcome=%s; lost=%d\n", observation.State, observation.Outcome, observation.Lost)
	fmt.Println("Selected owner socket; association with any stored command run is unproven.")
	for _, record := range observation.Records {
		fmt.Printf("%s %s elapsed=%.3fms", record.Stage, record.State, float64(record.Elapsed)/1e6)
		if record.Duration != nil {
			fmt.Printf(" duration=%.3fms", float64(*record.Duration)/1e6)
		}
		if record.Budget != nil {
			fmt.Printf(" remaining-context-budget=%.3fms", float64(*record.Budget)/1e6)
		} else if record.State == "started" {
			fmt.Print(" remaining-context-budget=unknown")
		}
		if record.ContextStop != "" {
			fmt.Printf(" context-stop=%s", record.ContextStop)
		}
		fmt.Println()
	}
	fmt.Printf("Stages without observed completion: %s\n", strings.Join(observation.Missing, ", "))
	return nil
}
