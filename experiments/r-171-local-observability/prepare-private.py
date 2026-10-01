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
        ('Потерянные байты доставки', '{job="node",__name__=~"diagnostic_selected_(queue_dropped_bytes|log_lost_bytes|console_dropped_bytes)_total"}', 'bytes'),
        ('Алерты: ожидание и срабатывание', 'ALERTS{alertstate=~"pending|firing"}', 'short'),
    ]
    for index,(title,expr,unit) in enumerate(specs):
        panels.append({'id':index+1,'title':title,'type':'timeseries',
            'gridPos':{'x':(index%3)*8,'y':4+(index//3)*8,'w':8,'h':8},'maxDataPoints':2000,
            'datasource':{'type':'prometheus','uid':'prometheus'},
            'targets':[{'refId':'A','expr':expr,'interval':'2s','legendFormat':title if index<3 else ('{{alertname}} · {{alertstate}}' if index==5 else '{{__name__}}')}],
            'fieldConfig':{'defaults':{'unit':unit,'custom':{'spanNulls':False}},'overrides':[]}})
    panels.append({'id':7,'title':'События настоящего Node','type':'logs',
        'gridPos':{'x':0,'y':20,'w':24,'h':12},'datasource':{'type':'loki','uid':'loki'},
        'targets':[{'refId':'A','expr':'{job="node"} | json | line_format "{{.entry_kind}} · {{.entry_state}}{{if .entry_failure}} · {{.entry_failure}}{{end}} · {{.stream}} #{{.sequence}}"'}],
        'options':{'showTime':True,'showLabels':False,'wrapLogMessage':True},
        'description':'Safe projected lifecycle events. Resource samples are separate metrics. Explore opens the original record and field filters.'})
    panels.append({'id':8,'title':'Источник и границы наблюдения','type':'text',
        'gridPos':{'x':0,'y':0,'w':24,'h':4},
        'options':{'mode':'markdown','content':'Настоящий Introduction Node и два Sources в одном локальном контейнере. **CPU и память относятся ко всему контейнеру.** Работающий процесс не доказывает готовность сети. Разрывы измерений не заполняются нулями. Go runtime/профили пока не подключены. Стенд работает один час. Подключение установленных узлов ещё не проверено.'}})
    (grafana/'dashboards'/'probe.json').write_text(json.dumps({'uid':'accepted-node',
        'title':'Ardents · живой Node','schemaVersion':39,'version':1,'editable':False,
        'time':{'from':'now-15m','to':'now'},'refresh':'5s','panels':panels},ensure_ascii=False))
    (provisioning/'dashboards'/'local.yml').write_text(json.dumps({'apiVersion':1,'providers':[
        {'name':'Actual local Node','type':'file','disableDeletion':True,'editable':False,
         'options':{'path':'/private/dashboards'}}]}))
