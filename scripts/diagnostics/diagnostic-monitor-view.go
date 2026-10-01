//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type monitorView struct {
	server   *http.Server
	listener net.Listener
	done     chan struct{}
	outcome  error
}

// The panel reads the selected supervisor's memory, never raw files or another
// process. A file-sink outage therefore does not hide its independent health.
func monitorHandler(delivery *monitorDelivery) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		io.WriteString(w, monitorDashboard)
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		state := delivery.snapshot()
		age := time.Since(state.Updated).Seconds()
		if age < 0 {
			age = 0
		}
		var outputAge *float64
		if state.LastOutput != nil {
			value := time.Since(*state.LastOutput).Seconds()
			if value < 0 {
				value = 0
			}
			outputAge = &value
		}
		status := "not-started"
		if state.StartFailed {
			status = "start-failed"
		} else if state.SourceAlive {
			status = "running"
		} else if state.SourceExit != nil || state.Finished != nil {
			status = "stopped"
		}
		if state.Finished == nil && age > 3 {
			status = "collector-stale"
		}
		body, err := json.Marshal(struct {
			State               monitorState `json:"state"`
			SourceStatus        string       `json:"source_status"`
			CollectorAgeSeconds float64      `json:"collector_age_seconds"`
			OutputAgeSeconds    *float64     `json:"output_age_seconds"`
		}{state, status, age, outputAge})
		if err != nil || len(body) > 65536 {
			http.Error(w, "monitor status unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		body, err := monitorMetrics(delivery.snapshot(), time.Now().UTC())
		if err != nil {
			http.Error(w, "monitor metrics unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write(body)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		host, _, err := net.SplitHostPort(r.Host)
		ip := net.ParseIP(host)
		if err != nil || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
			http.Error(w, "local panel host required", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Scheme != "http" || parsed.Host != r.Host || parsed.Path != "" {
				http.Error(w, "same origin required", http.StatusForbidden)
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "same origin required", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func openMonitorView(address string, container bool, delivery *monitorDelivery) (*monitorView, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	numericPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || ip == nil || !ip.IsLoopback() && !(container && host == "0.0.0.0") || numericPort > 65535 {
		return nil, errors.New("monitor panel requires loopback or explicit container binding")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	bounded := &monitorListener{Listener: listener, slots: make(chan struct{}, 4)}
	view := &monitorView{listener: bounded, done: make(chan struct{})}
	view.server = &http.Server{Handler: monitorHandler(delivery), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192}
	go func() {
		outcome := view.server.Serve(bounded)
		if errors.Is(outcome, http.ErrServerClosed) {
			outcome = nil
		}
		view.outcome = outcome
		if outcome != nil {
			delivery.mu.Lock()
			delivery.state.PanelFailed = true
			delivery.mu.Unlock()
		}
		close(view.done)
	}()
	return view, nil
}
func (v *monitorView) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := v.server.Shutdown(ctx)
	if err != nil {
		err = errors.Join(err, v.server.Close())
	}
	select {
	case <-v.done:
		return errors.Join(err, v.outcome)
	case <-ctx.Done():
		return errors.Join(err, ctx.Err())
	}
}

type monitorListener struct {
	net.Listener
	slots chan struct{}
}

func (l *monitorListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			return &monitorConn{Conn: conn, slots: l.slots}, nil
		default:
			conn.Close()
		}
	}
}

type monitorConn struct {
	net.Conn
	slots chan struct{}
	once  sync.Once
}

func (c *monitorConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { <-c.slots })
	return err
}

const monitorDashboard = `<!doctype html><html lang="ru"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Ardents · Мониторинг</title>
<style>
:root{color-scheme:dark;font:15px system-ui;background:#0b1220;color:#e5edf8}*{box-sizing:border-box}body{margin:0}main{max-width:1220px;margin:auto;padding:28px}header{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:22px}h1{font-size:26px;margin:6px 0}h2{font-size:18px;margin:0}p{color:#aebed4;line-height:1.5}section{background:#111e30;border:1px solid #26374e;border-radius:12px;padding:20px;margin:18px 0}.grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.cell{padding:12px;background:#0c1728;border-radius:8px}.label{font-size:13px;color:#aebed4}.value{font-size:19px;margin-top:8px}.toolbar{display:flex;flex-wrap:wrap;align-items:center;gap:12px;margin:16px 0}button,select,input{font:inherit;color:inherit;background:#1b2e46;border:1px solid #3b526e;border-radius:6px;padding:8px 12px}button{cursor:pointer}button:hover{background:#294568}input{min-width:220px}table{width:100%;border-collapse:collapse;font-size:14px}td,th{text-align:left;vertical-align:top;padding:10px 8px;border-bottom:1px solid #26374e}th{color:#aebed4;font-weight:500}code{white-space:pre-wrap;overflow-wrap:anywhere}.table{overflow:auto}.fault{color:#ffb2ab}.muted{color:#aebed4}.right{text-align:right}details{margin-top:8px}summary{cursor:pointer;color:#9cc7ff}small{color:#aebed4}@media(max-width:750px){main{padding:16px}.grid{grid-template-columns:1fr}header{align-items:flex-start;flex-direction:column}.table{max-width:100%}}
</style><main>
<header><div><small>ARDENTS / LOCAL MONITORING</small><h1>Мониторинг · живые логи</h1><div id="source" class="muted">Ожидаем выбранный источник</div></div><button id="pause">Пауза просмотра</button></header>
<section><div class="grid"><div class="cell"><div class="label">Источник</div><div id="source-state" class="value">—</div><div id="source-result" class="muted"></div></div><div class="cell"><div class="label">Сборщик</div><div id="collector-state" class="value">—</div><div id="freshness" class="muted"></div></div><div class="cell"><div class="label">Файлы логов</div><div id="file-state" class="value">—</div><div id="retention" class="muted"></div></div></div><p id="faults" class="fault"></p><div id="delivery" class="muted"></div></section>
<section><h2>События процесса</h2><p>Периодические измерения ресурсов сохраняются отдельно и не занимают окно событий. <span id="samples"></span></p><div class="toolbar"><label>Поток <select id="stream"><option value="">Все</option><option>stdout</option><option>stderr</option></select></label><label>Категория <select id="kind"><option value="">Все</option></select></label><input id="search" placeholder="Фильтр видимых записей" aria-label="Фильтр логов"><button id="refresh">Обновить</button></div><p id="window">Последние записи: до 64 строк и 48 KiB. Пауза останавливает только просмотр.</p><div class="table"><table><thead><tr><th>№</th><th>Время</th><th>Компонент / поток</th><th>Событие</th></tr></thead><tbody id="rows"></tbody></table></div><p id="empty">Ожидаем события</p></section>
<p>Панель показывает фиксированные категории и числовые наблюдения выбранного локального процесса. Содержимое stdout/stderr и приватные файлы через HTTP не выдаются. Работающий процесс не доказывает готовность узла.</p>
</main><script>
const el=id=>document.getElementById(id),text=(id,value)=>el(id).textContent=value;let paused=false,busy=false,current=null,lastSeen=0,viewMissed=0;
function bytes(n){if(n<1024)return n+' B';if(n<1048576)return (n/1024).toFixed(1)+' KiB';return (n/1048576).toFixed(1)+' MiB'}
const owners={'ardents-node-event-v1':'Node','ardents-source-event-v1':'Source','ardents-headless-runtime-event-v1':'Headless runtime'};
const descriptions={'lifecycle':'Изменение состояния узла','resource':'Изменение состояния ресурсов','source-ready':'Source готов','source-wave-accepted':'Source принял волну работы','source-failed':'Отказ Source','headless-runtime-ready':'Среда выполнения готова','headless-runtime-failed':'Отказ среды выполнения','headless-runtime-permission-required':'Требуется разрешение','headless-runtime-publication-refresh-failed':'Отказ обновления публикации','headless-runtime-publication-withdrawal-failed':'Отказ отзыва публикации','headless-runtime-connection-operation-failed':'Отказ операции соединения','unstructured-output':'Нераспознанная строка; содержимое скрыто'};
function rows(){if(!current)return;const state=current.state,body=el('rows');body.replaceChildren();let visible=0;for(const r of state.tail||[]){if(el('stream').value&&r.stream!==el('stream').value)continue;if(el('kind').value&&r.entry.kind!==el('kind').value)continue;const summary=[descriptions[r.entry.kind]||r.entry.kind,r.entry.state,r.entry.failure,r.entry.carrier,r.entry.kind].filter(Boolean).join(' · ');if(el('search').value&&!summary.toLowerCase().includes(el('search').value.toLowerCase()))continue;visible++;let row=document.createElement('tr');for(const v of [r.sequence,new Date(r.at).toLocaleTimeString('ru-RU',{hour12:false})+'.'+String(new Date(r.at).getMilliseconds()).padStart(3,'0'),(owners[r.entry.schema]||'Источник не определён')+' / '+r.stream,summary]){let cell=document.createElement('td');cell.textContent=v;row.append(cell)}if(r.entry.resource||r.entry.hosting){let details=document.createElement('details'),label=document.createElement('summary'),code=document.createElement('code');label.textContent='Числовые поля';code.textContent=JSON.stringify({resource:r.entry.resource,hosting:r.entry.hosting},null,2);details.append(label,code);row.lastChild.append(details)}body.append(row)}text('empty',visible?'':'Нет записей для выбранного фильтра');text('window','Показано '+visible+' из '+(state.tail||[]).length+' последних записей · всего '+state.total_rows+'. '+(viewMissed?'Вне окна просмотра: '+viewMissed+'. ':'')+'Пауза останавливает только просмотр.')}
function render(data){current=data;let s=data.state;text('samples','Измерений: '+s.total_samples+(s.latest_sample?' · последнее: '+new Date(s.latest_sample.at).toLocaleTimeString('ru-RU'):' · ещё нет наблюдений'));text('source',s.source_name+' · PID '+(s.source_pid||'не запущен')+' · с '+new Date(s.started).toLocaleString('ru-RU'));text('source-state',{'running':'Процесс работает','stopped':'Процесс завершён','start-failed':'Отказ запуска','not-started':'Ожидаем запуск','collector-stale':'Нет свежих наблюдений'}[data.source_status]||'Недоступно');text('source-result',s.source_exit===undefined?'Exit code ещё не наблюдён':'Exit code: '+s.source_exit+(s.timed_out?' · таймаут':s.interrupted?' · прерывание':''));text('collector-state',s.cleanup_in_progress?'Завершаем сбор':s.retention_active?'Обслуживаем сохранённые логи':s.finished?'Сбор завершён':'Сбор выполняется');text('freshness','Снимок: '+data.collector_age_seconds.toFixed(1)+' с назад · вывод: '+(data.output_age_seconds===null?'ещё не наблюдался':data.output_age_seconds.toFixed(1)+' с назад'));text('file-state',s.file_failed?'Ошибка записи':bytes(s.logs.retained_bytes)+' / '+bytes(s.limits.max_bytes));text('retention',s.logs.files+' / '+s.limits.max_files+' файлов · удалено по сроку/лимиту '+bytes(s.logs.expired_bytes));let faults=[];if(s.retention_failed)faults.push('Возрастное удаление логов: отказ');if(s.console_failed)faults.push('Консоль: отказ');if(s.file_failed)faults.push('Файлы: отказ');if(s.snapshot_failed)faults.push('Сохранение состояния: отказ');if(s.cleanup_failed)faults.push('Завершение: ошибки сохранены');if(s.panel_failed)faults.push('HTTP: отказ');const lost=s.queue_dropped_bytes+s.console_dropped_bytes+s.logs.lost_bytes;if(lost)faults.push('Есть потери доставки; счётчики каждого канала приведены ниже');text('faults',faults.join(' · '));text('delivery','Очередь файлов: потеряно '+bytes(s.queue_dropped_bytes)+' · запись файлов: '+bytes(s.logs.lost_bytes)+' · консоль: '+bytes(s.console_dropped_bytes)+' · неструктурированных строк: '+s.unknown_lines+' · слишком длинных: '+s.oversized_lines);const firstRetained=(s.tail||[])[0]?.sequence;if(lastSeen&&firstRetained>lastSeen+1)viewMissed+=firstRetained-lastSeen-1;lastSeen=s.total_rows;const selected=el('kind').value,kinds=[...new Set((s.tail||[]).map(r=>r.entry.kind))].sort();el('kind').replaceChildren();for(const k of ['',...kinds]){let o=document.createElement('option');o.value=k;o.textContent=k||'Все';el('kind').append(o)}if(kinds.includes(selected))el('kind').value=selected;rows()}
async function update(force=false){if(busy||(paused&&!force))return;busy=true;try{const response=await fetch('/status',{cache:'no-store',signal:AbortSignal.timeout(3000)});if(!response.ok)throw Error();render(await response.json())}catch{text('source-state','Текущее состояние неизвестно');text('source-result','Связь со сборщиком потеряна');text('freshness','Сохранённый снимок от '+(current?new Date(current.state.updated).toLocaleTimeString('ru-RU'):'—'));text('collector-state','Источник наблюдений недоступен');text('faults','Не удалось получить свежий снимок. Последние записи оставлены на экране.')}finally{busy=false}}
el('pause').onclick=()=>{paused=!paused;text('pause',paused?'Продолжить просмотр':'Пауза просмотра');if(paused)text('freshness','Просмотр приостановлен · снимок от '+(current?new Date(current.state.updated).toLocaleTimeString('ru-RU'):'—'));else update()};el('refresh').onclick=()=>update(true);el('stream').onchange=rows;el('kind').onchange=rows;el('search').oninput=rows;update();setInterval(update,1000);
</script></html>`

// monitorMetrics exposes only fixed safe signals from supervisor memory.
// It reads no files and starts no sampling or profiling operation.
func monitorMetrics(state monitorState, now time.Time) ([]byte, error) {
	if state.Schema != "ardents-monitor-v1" || state.Started.IsZero() || state.Updated.IsZero() ||
		state.Updated.After(now) || state.Started.After(state.Updated) ||
		state.MetricSampleMaxAge < 0 || state.MetricSampleMaxAge > time.Hour {
		return nil, errors.New("invalid metric observation interval")
	}
	var output strings.Builder
	emit := func(name, kind, help string, value float64) {
		name = "diagnostic_selected_" + name
		fmt.Fprintf(&output, "# HELP %s %s\n# TYPE %s %s\n%s %g\n", name, help, name, kind, name, value)
	}
	boolean := func(value bool) float64 {
		if value {
			return 1
		}
		return 0
	}
	age := now.Sub(state.Updated)
	fresh := age <= 3*time.Second
	emit("monitor_fresh", "gauge", "Supervisor heartbeat within three seconds.", boolean(fresh))
	emit("monitor_observation_age_seconds", "gauge", "Supervisor heartbeat age.", age.Seconds())
	emit("session_started_seconds", "gauge", "Supervisor session start; counter reset boundary.", float64(state.Started.UnixNano())/1e9)
	emit("resource_export_enabled", "gauge", "Explicit producer freshness budget configured.", boolean(state.MetricSampleMaxAge > 0))
	if fresh {
		if state.SourcePID > 0 || state.StartFailed || state.SourceExit != nil {
			emit("source_process_alive", "gauge", "Observed process survival; not product readiness.", boolean(state.SourceAlive))
		}
		for _, signal := range []struct {
			name, help string
			value      bool
		}{
			{"log_file_failed", "Observed retained log sink failure.", state.FileFailed},
			{"console_failed", "Observed projected console failure.", state.ConsoleFailed},
			{"snapshot_failed", "Observed independent snapshot writer failure.", state.SnapshotFailed},
			{"metrics_listener_failed", "Observed protected collector listener failure.", state.MetricsFailed},
			{"retention_failed", "Observed retained log pruning failure.", state.RetentionFailed},
			{"cleanup_failed", "Observed owned cleanup failure.", state.CleanupFailed},
		} {
			emit(signal.name, "gauge", signal.help, boolean(signal.value))
		}
		for _, signal := range []struct {
			name, help string
			value      int64
		}{
			{"queue_dropped_bytes_total", "Session bytes lost at delivery queues.", state.QueueDroppedBytes},
			{"console_dropped_bytes_total", "Session bytes lost at console delivery.", state.ConsoleDroppedBytes},
			{"log_lost_bytes_total", "Session bytes lost at retained log delivery.", state.Logs.LostBytes},
			{"log_expired_bytes_total", "Session bytes intentionally expired by retention; not failed delivery.", state.Logs.ExpiredBytes},
		} {
			if signal.value < 0 {
				return nil, errors.New("invalid delivery metric")
			}
			emit(signal.name, "counter", signal.help, float64(signal.value))
		}
	}
	// Limits originate from the admitted log store, not producer resource fields.
	// These are payload accounting gauges; they do not measure physical disk use.
	storageFresh := !state.LogsObservedAt.IsZero() && !state.LogsObservedAt.After(now) && !state.LogsObservedAt.Before(state.Started) && now.Sub(state.LogsObservedAt) <= 3*time.Second
	storageAvailable := fresh && storageFresh && state.Limits.SegmentBytes > 0 && state.Limits.MaxBytes >= state.Limits.SegmentBytes && state.Limits.MaxFiles >= 2
	emit("log_storage_observation_available", "gauge", "Fresh admitted log payload accounting; not filesystem allocation.", boolean(storageAvailable))
	if storageAvailable {
		if state.Logs.RetainedBytes < 0 || state.Logs.RetainedBytes > state.Limits.MaxBytes || state.Logs.Files < 1 || state.Logs.Files > state.Limits.MaxFiles {
			return nil, errors.New("invalid retained log accounting")
		}
		emit("log_retained_bytes", "gauge", "Observed retained log payload bytes; excludes filesystem metadata and status files.", float64(state.Logs.RetainedBytes))
		emit("log_retained_files", "gauge", "Observed owned log-directory files including the empty ownership lock.", float64(state.Logs.Files))
		emit("log_retention_limit_bytes", "gauge", "Configured retained payload budget; not filesystem quota.", float64(state.Limits.MaxBytes))
		emit("log_retention_limit_files", "gauge", "Configured file budget including the empty ownership lock.", float64(state.Limits.MaxFiles))
	}
	filesystemAvailable := storageAvailable && state.Logs.FilesystemAvailable
	emit("log_filesystem_observation_available", "gauge", "Fresh capacity observation of the filesystem containing the selected logs.", boolean(filesystemAvailable))
	if filesystemAvailable {
		if state.Logs.FilesystemTotalBytes <= 0 || state.Logs.FilesystemFreeBytes < 0 || state.Logs.FilesystemFreeBytes > state.Logs.FilesystemTotalBytes {
			return nil, errors.New("invalid log filesystem capacity")
		}
		emit("log_filesystem_total_bytes", "gauge", "Whole log filesystem capacity; may be tmpfs, not host disk or Node allocation.", float64(state.Logs.FilesystemTotalBytes))
		emit("log_filesystem_available_bytes", "gauge", "Whole log filesystem bytes available to the monitor; includes shared usage and reserved capacity.", float64(state.Logs.FilesystemFreeBytes))
	}
	available := false
	row := state.LatestSample
	if fresh && state.SourceAlive && state.MetricSampleMaxAge > 0 && row != nil &&
		row.Entry.Schema == "ardents-node-event-v1" && row.Entry.Kind == "resource-sample" {
		producer, err := time.Parse(time.RFC3339Nano, row.Entry.At)
		if err != nil || producer.After(now) || row.At.After(now) ||
			producer.Before(state.Started) || row.At.Before(state.Started) {
			return nil, errors.New("invalid selected producer observation")
		}
		emit("sample_age_seconds", "gauge", "Selected producer observation age.", now.Sub(producer).Seconds())
		available = now.Sub(producer) <= state.MetricSampleMaxAge && now.Sub(row.At) <= state.MetricSampleMaxAge
		if available {
			// Both current Node samplers measure cgroup CPU and memory. The event
			// schema does not identify which sampler populated other fields;
			// their encoded defaults are not availability evidence.
			for _, signal := range []struct {
				field, name, kind, help string
				scale                   float64
			}{
				{"cpu_usage_usec", "cgroup_cpu_usage_seconds_total", "counter", "Selected cgroup cumulative CPU time; reset breaks continuity.", 1e-6},
				{"memory_bytes", "cgroup_memory_bytes", "gauge", "Selected cgroup current memory; not process RSS.", 1},
			} {
				value, ok := row.Entry.Resource[signal.field]
				if !ok {
					continue
				}
				if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
					return nil, errors.New("invalid selected resource metric")
				}
				emit(signal.name, signal.kind, signal.help, value*signal.scale)
			}
		}
	}
	emit("sample_fresh", "gauge", "Fresh selected Node observation; missing fields remain absent.", boolean(available))
	if output.Len() > 16<<10 {
		return nil, errors.New("metric response exceeds bound")
	}
	return []byte(output.String()), nil
}
