"""Prepare private, disposable synthetic monitoring credentials and provisioning."""
import json
import os
import pathlib
import secrets
import shutil
import subprocess

root = pathlib.Path('/private')
os.umask(0o077)
node_preview = os.environ.get('R171_SOURCE_MODE') == 'accepted-node'
history_trial = os.environ.get('R171_ALERT_HISTORY_TRIAL') == '1'
if history_trial and node_preview:
    raise RuntimeError('History trial requires the synthetic source')
roles = ('prometheus', 'alertmanager', 'loki', 'alloy', 'grafana', 'query') + (('node',) if node_preview else ())
ca = root / 'ca'
ca.mkdir(mode=0o700)
def openssl(*args):
    subprocess.run(['openssl', *map(str,args)], check=True, stdout=subprocess.DEVNULL,
                   stderr=subprocess.PIPE, timeout=15)
openssl('req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj','/CN=Local synthetic probe CA',
        '-keyout',ca/'ca.key','-out',ca/'ca.crt')
for role in roles:
    directory = root / role
    directory.mkdir(mode=0o700)
    shutil.copyfile(ca/'ca.crt', directory/'ca.crt')
    (directory/'extensions.txt').write_text(f'subjectAltName=DNS:{role}\nextendedKeyUsage=serverAuth,clientAuth\n')
    openssl('req','-new','-newkey','rsa:2048','-nodes','-subj',f'/CN={role}',
            '-keyout',directory/'client.key','-out',directory/'request.csr')
    openssl('x509','-req','-days','1','-in',directory/'request.csr','-CA',ca/'ca.crt',
            '-CAkey',ca/'ca.key','-set_serial',roles.index(role)+1,
            '-extfile',directory/'extensions.txt','-out',directory/'client.crt')
    if role in ('prometheus','alertmanager','loki','node'):
        shutil.copyfile(directory/'client.key',directory/'server.key')
        shutil.copyfile(directory/'client.crt',directory/'server.crt')
    if role in ('prometheus','alertmanager'):
        (directory/'web.yml').write_text('tls_server_config:\n  cert_file: /certs/server.crt\n'
            '  key_file: /certs/server.key\n  client_auth_type: RequireAndVerifyClientCert\n'
            '  client_ca_file: /certs/ca.crt\n  min_version: TLS12\n')
grafana = root/'grafana'
password = secrets.token_hex(24)
(grafana/'password').write_text(password)
(grafana/'secret-key').write_text(secrets.token_hex(32))
(root/'query'/'grafana-login.json').write_text(json.dumps({'user':'probe','password':password}))
provisioning = grafana/'provisioning'
(provisioning/'datasources').mkdir(parents=True)
(provisioning/'dashboards').mkdir()
(provisioning/'plugins').mkdir()
(provisioning/'alerting').mkdir()
datasources = []
for name,kind,url in [('Metrics','prometheus','https://prometheus:9090'),('Logs','loki','https://loki:3100')]:
    datasources.append({'name':name,'uid':kind,'type':kind,'access':'proxy','url':url,
        'editable':False,'jsonData':{'tlsAuth':True,'tlsAuthWithCACert':True,'serverName':kind if kind=='loki' else 'prometheus'},
        'secureJsonData':{'tlsCACert':(grafana/'ca.crt').read_text(),
                         'tlsClientCert':(grafana/'client.crt').read_text(),
                         'tlsClientKey':(grafana/'client.key').read_text()}})

# Explicit isolated history trial. Normal Prometheus-evaluated Node rules stay
# unchanged; this rule is supplied only by the selected synthetic profile.
if history_trial:
    history_rule = json.loads((pathlib.Path('/probe')/'alert-history-trial.json').read_text())
    (provisioning/'alerting'/'history.yml').write_text(json.dumps(history_rule))
# JSON is valid YAML; no external serializer or secret environment variables.
(provisioning/'datasources'/'local.yml').write_text(json.dumps({'apiVersion':1,'datasources':datasources}))
(provisioning/'dashboards'/'local.yml').write_text(json.dumps({'apiVersion':1,'providers':[
    {'name':'Synthetic probe','type':'file','disableDeletion':True,'editable':False,
     'options':{'path':'/private/dashboards'}}]}))
(grafana/'dashboards').mkdir()
panels = []
for i,(title,expr,unit) in enumerate([
    ('Fixture queue (items)','diagnostic_fixture_queue_items','short'),
    ('Fixture events (total)','diagnostic_fixture_events_total','short'),
    ('Source / collector availability','up{job=~"fixture|collector"}','short')]):
    panels.append({'id':i+1,'title':title,'type':'timeseries','gridPos':{'x':i*8,'y':0,'w':8,'h':8},
      'datasource':{'type':'prometheus','uid':'prometheus'},'targets':[{'refId':'A','expr':expr,'legendFormat':'{{job}}' if expr.startswith('up{') else ('Queue' if i==0 else 'Events total')}],
      'fieldConfig':{'defaults':{'unit':unit,'custom':{'spanNulls':False}},'overrides':[]}})
panels.append({'id':5,'title':'Source observation age (seconds)',
    'description':'Age of the fixture observation, independent of log silence. Missing series means unavailable.',
    'type':'timeseries','gridPos':{'x':0,'y':8,'w':8,'h':8},
    'datasource':{'type':'prometheus','uid':'prometheus'},
    'targets':[{'refId':'A','expr':'time() - diagnostic_fixture_heartbeat_seconds','legendFormat':'Observation age'}],
    'fieldConfig':{'defaults':{'unit':'s','custom':{'spanNulls':False}},'overrides':[]}})
panels.append({'id':6,'title':'Alert history: pending / firing',
    'description':'Measured Prometheus rule states. Absence alone is not proof of recovery; check source and collector availability. Silence does not change this history.',
    'type':'timeseries','gridPos':{'x':8,'y':8,'w':16,'h':8},
    'datasource':{'type':'prometheus','uid':'prometheus'},
    'targets':[{'refId':'A','expr':'ALERTS{alertstate=~"pending|firing"}',
                'legendFormat':'{{alertname}} · {{alertstate}} · {{scope}}'}],
    'fieldConfig':{'defaults':{'unit':'short','custom':{'spanNulls':False}},'overrides':[]}})
collector=os.environ.get('R171_COLLECTOR','alloy')
if collector not in ('alloy','otel'): raise RuntimeError('Unknown selected collector')
parse_stage='' if collector=='otel' else ' | json'
log_expression='{job="fixture"}'+parse_stage+' | line_format "{{.scope}} · {{.event}} #{{.sequence}}"'
panels.append({'id':4,'title':'Events: severity / source / message','type':'logs',
    'gridPos':{'x':0,'y':16,'w':24,'h':10},'datasource':{'type':'loki','uid':'loki'},
    'description':'Synthetic events only. Query formatting does not rewrite stored JSON. Use Explore for field search and original record details.',
    'targets':[{'refId':'A','expr':log_expression}],
    'options':{'showTime':True,'showLabels':False,'wrapLogMessage':True}})
(grafana/'dashboards'/'probe.json').write_text(json.dumps({'uid':'synthetic-probe','title':'Local synthetic monitoring probe',
 'schemaVersion':39,'version':1,'editable':False,'time':{'from':'now-5m','to':'now'},'refresh':'5s','panels':panels}))
print('Prepared separate role keys and local data sources; no credentials printed')

if node_preview:
    node_log_format = (
        '{{if eq .entry_schema "ardents-node-event-v1"}}Node'
        '{{else if eq .entry_schema "ardents-source-event-v1"}}Source'
        '{{else if eq .entry_schema "ardents-headless-runtime-event-v1"}}Endpoint'
        '{{else}}Collector{{end}} · {{if eq .entry_kind "lifecycle"}}Переход состояния'
        '{{else if eq .entry_kind "resource"}}Решение по ресурсам'
        '{{else if eq .entry_kind "source-ready"}}Source готов'
        '{{else if eq .entry_kind "source-wave-accepted"}}Волна принята'
        '{{else if eq .entry_kind "source-failed"}}Source сообщил об отказе'
        '{{else if eq .entry_kind "headless-runtime-connection-operation-failed"}}Операция соединения отказала'
        '{{else if eq .entry_kind "headless-runtime-publication-refresh-failed"}}Обновление публикации отказало'
        '{{else if eq .entry_kind "headless-runtime-publication-withdrawal-failed"}}Снятие публикации отказало'
        '{{else if eq .entry_kind "headless-runtime-permission-required"}}Требуется разрешение'
        '{{else if eq .entry_kind "headless-runtime-ready"}}Endpoint готов'
        '{{else if eq .entry_kind "headless-runtime-failed"}}Endpoint сообщил об отказе'
        '{{else}}{{.entry_kind}}{{end}}{{if .entry_state}} · {{.entry_state}}{{end}}{{if .entry_failure}} · категория отказа: {{.entry_failure}}{{end}}{{if .entry_carrier}} · {{.entry_carrier}}{{end}} · {{.stream}} #{{.sequence}}'
    )
    node_log_query = ('{job="node"} | json | entry_schema=~"${component:regex}" '
                      '| entry_kind=~"${event_kind:regex}" | line_format '
                      + json.dumps(node_log_format, ensure_ascii=False))
    node = root / 'node'
    shutil.copyfile(ca / 'ca.crt', node / 'client-ca.crt')
    public = subprocess.run(['openssl', 'x509', '-in', str(root/'prometheus'/'client.crt'),
                             '-pubkey', '-noout'], capture_output=True, check=True, timeout=5).stdout
    der = subprocess.run(['openssl', 'pkey', '-pubin', '-outform', 'DER'],
                         input=public, capture_output=True, check=True, timeout=5).stdout
    import hashlib
    (node/'client-pin.txt').write_text(hashlib.sha256(der).hexdigest())
    (root/'source-reports').mkdir(mode=0o700)
    measured = ' and on(job,instance) (up{job="node"} == 1) and on(job,instance) (diagnostic_selected_sample_fresh{job="node"} == 1) and on(job,instance) (diagnostic_selected_source_process_alive{job="node"} == 1)'
    panels = []
    specs = [
        ('Память общего контейнера', 'diagnostic_selected_cgroup_memory_bytes{job="node"}'+measured, 'bytes'),
        ('CPU общего контейнера', '(rate(diagnostic_selected_cgroup_cpu_usage_seconds_total{job="node"}[30s]) and on(job,instance) (count_over_time(diagnostic_selected_cgroup_cpu_usage_seconds_total{job="node"}[30s]) >= 15) and on(job,instance) (changes(diagnostic_selected_session_started_seconds{job="node"}[30s]) == 0))'+measured, 'cores'),
        ('Возраст ресурсного измерения', 'diagnostic_selected_sample_age_seconds{job="node"}', 's'),
        ('Процесс Node и доступность сбора', '{job="node",__name__=~"up|diagnostic_selected_source_process_alive|diagnostic_selected_monitor_fresh|diagnostic_selected_sample_fresh"}', 'short'),
        ('Потери локального сборщика (байты)', '{job="node",__name__=~"diagnostic_selected_(queue_dropped_bytes|log_lost_bytes|console_dropped_bytes)_total"}', 'bytes'),
        ('Алерты: ожидание и срабатывание', 'ALERTS{alertstate=~"pending|firing"}', 'short'),
        ('Коллектор: окончательно не отправлено', 'sum(otelcol_exporter_send_failed_log_records{job="collector",exporter="otlp_http/fixture"})', 'short'),
        ('Коллектор: очередь доставки', 'sum(otelcol_exporter_queue_size{job="collector",exporter="otlp_http/fixture",data_type="logs"})', 'bytes'),
    ]
    for index,(title,expr,unit) in enumerate(specs):
        panels.append({'id':index+1,'title':title,'type':'timeseries',
            'gridPos':{'x':(index%3)*8,'y':4+(index//3)*8,'w':8,'h':8},'maxDataPoints':2000,
            'datasource':{'type':'prometheus','uid':'prometheus'},
            'targets':[{'refId':'A','expr':expr,'interval':'2s','legendFormat':title if index<3 or index>=6 else ('{{alertname}} · {{alertstate}}' if index==5 else '{{__name__}}')}],
            'fieldConfig':{'defaults':{'unit':unit,'custom':{'spanNulls':False}},'overrides':[]}})
    storage_measured = ' and on(job,instance) (up{job="node"} == 1) and on(job,instance) (diagnostic_selected_monitor_fresh{job="node"} == 1) and on(job,instance) (diagnostic_selected_log_storage_observation_available{job="node"} == 1)'
    storage_specs = [
        (11, 'Журнал: сохранённые байты и лимит', 'bytes', [('log_retained_bytes', 'Сохранено'), ('log_retention_limit_bytes', 'Лимит')], 'Учёт содержимого локальных файлов. Метаданные файловой системы, status-файлы и журналы Docker сюда не входят. Лимит не является квотой диска.'),
        (12, 'Журнал: файлы и лимит', 'short', [('log_retained_files', 'Файлов'), ('log_retention_limit_files', 'Лимит')], 'Включает пустой файл блокировки владельца. Отсутствующее или устаревшее наблюдение не заменяется нулём.'),
        (13, 'Журнал: удалено по сроку или лимиту', 'bytes', [('log_expired_bytes_total', 'Удалено за сессию')], 'Плановая ротация, отдельно от потерь доставки. Счётчик сбрасывается при перезапуске монитора; удалённые сообщения не восстанавливаются.'),
    ]
    for index,(panel_id,title,unit,signals,description) in enumerate(storage_specs):
        panels.append({'id':panel_id,'title':title,'type':'timeseries',
            'gridPos':{'x':index*8,'y':40,'w':8,'h':8},'maxDataPoints':2000,
            'datasource':{'type':'prometheus','uid':'prometheus'},
            'targets':[{'refId':chr(65+offset),'expr':'diagnostic_selected_'+metric+'{job="node"}'+storage_measured,'interval':'2s','legendFormat':label} for offset,(metric,label) in enumerate(signals)],
            'description':description,'fieldConfig':{'defaults':{'unit':unit,'custom':{'spanNulls':False}},'overrides':[]}})
    panels.append({'id':14,'title':'Файловая система журнала: заполнение','type':'timeseries',
        'gridPos':{'x':0,'y':48,'w':12,'h':8},'maxDataPoints':2000,
        'datasource':{'type':'prometheus','uid':'prometheus'},
        'targets':[{'refId':'A','expr':'(1 - diagnostic_selected_log_filesystem_available_bytes{job="node"} / diagnostic_selected_log_filesystem_total_bytes{job="node"}) and on(job,instance) (diagnostic_selected_log_filesystem_observation_available{job="node"} == 1)','legendFormat':'Заполнение'}],
        'description':'Вся файловая система, содержащая журнал: учитывает другие файлы и резерв. В контейнерном стенде это может быть tmpfs, а не физический диск хоста. Нет измерения — разрыв графика.',
        'fieldConfig':{'defaults':{'unit':'percentunit','min':0,'max':1,'custom':{'spanNulls':False},'thresholds':{'mode':'absolute','steps':[{'color':'green','value':None},{'color':'orange','value':0.8},{'color':'red','value':0.9}]}},'overrides':[]}})
    panels.append({'id':9,'title':'События настоящего Node','type':'logs',
        'gridPos':{'x':0,'y':28,'w':24,'h':12},'datasource':{'type':'loki','uid':'loki'},
        'targets':[{'refId':'A','expr':node_log_query}],
        'options':{'showTime':True,'showLabels':False,'wrapLogMessage':True,'sortOrder':'Descending','enableLogDetails':True},
        'description':'Фильтры компонента и события применяются к выбранному интервалу. Раскройте строку для исходной категории, времени владельца и времени наблюдения. READY — событие в прошлом, а не текущая готовность. Категория отказа не доказывает первопричину. Ресурсные samples показаны в метриках.'})
    panels.append({'id':10,'title':'Источник и границы наблюдения','type':'text',
        'gridPos':{'x':0,'y':0,'w':24,'h':4},
        'options':{'mode':'markdown','content':'Настоящий Introduction Node и два Sources в одном локальном контейнере. **CPU и память относятся ко всему контейнеру.** Работающий процесс не доказывает готовность сети. Разрывы измерений не заполняются нулями. Профили доступны отдельно в приватном debug режиме и не поступают в мониторинг. Стенд работает один час. Подключение установленных узлов ещё не проверено.'}})
    (grafana/'dashboards'/'probe.json').write_text(json.dumps({'uid':'accepted-node',
        'title':'Ardents · живой Node','schemaVersion':39,'version':1,'editable':False,
        'time':{'from':'now-15m','to':'now'},'refresh':'5s',
        'templating':{'list':[
            {'name':'component','label':'Компонент','type':'custom',
             'query':'Node : ardents-node-event-v1, Source : ardents-source-event-v1, Endpoint : ardents-headless-runtime-event-v1',
             'multi':False,'includeAll':True,'allValue':'.*',
             'current':{'text':'All','value':'$__all','selected':True}},
            {'name':'event_kind','label':'Событие','type':'custom',
             'query':'lifecycle, resource, source-ready, source-wave-accepted, source-failed, headless-runtime-ready, headless-runtime-failed, headless-runtime-permission-required, headless-runtime-publication-refresh-failed, headless-runtime-publication-withdrawal-failed, headless-runtime-connection-operation-failed',
             'multi':False,'includeAll':True,'allValue':'.*',
             'current':{'text':'All','value':'$__all','selected':True}}
        ]},'panels':panels},ensure_ascii=False))
    (provisioning/'dashboards'/'local.yml').write_text(json.dumps({'apiVersion':1,'providers':[
        {'name':'Actual local Node','type':'file','disableDeletion':True,'editable':False,
         'options':{'path':'/private/dashboards'}}]}))
