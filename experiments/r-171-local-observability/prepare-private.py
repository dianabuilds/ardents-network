"""Prepare private, disposable synthetic monitoring credentials and provisioning."""
import json
import os
import pathlib
import secrets
import shutil
import subprocess

root = pathlib.Path('/private')
os.umask(0o077)
roles = ('prometheus', 'alertmanager', 'loki', 'alloy', 'grafana', 'query')
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
    if role in ('prometheus','alertmanager','loki'):
        shutil.copyfile(directory/'client.key',directory/'server.key')
        shutil.copyfile(directory/'client.crt',directory/'server.crt')
    if role in ('prometheus','alertmanager'):
        (directory/'web.yml').write_text('tls_server_config:\n  cert_file: /certs/server.crt\n'
            '  key_file: /certs/server.key\n  client_auth_type: RequireAndVerifyClientCert\n'
            '  client_ca_file: /certs/ca.crt\n  min_version: TLS12\n')
grafana = root/'grafana'
password = secrets.token_hex(24)
(grafana/'password').write_text(password)
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
    ('Synthetic queue (items; not Node duty)','diagnostic_fixture_queue_items','short'),
    ('Generated fixture events (counter)','diagnostic_fixture_events_total','short'),
    ('Scrape availability (fixture/collector)','up{job=~"fixture|collector"}','short')]):
    panels.append({'id':i+1,'title':title,'type':'timeseries','gridPos':{'x':i*8,'y':0,'w':8,'h':8},
      'datasource':{'type':'prometheus','uid':'prometheus'},'targets':[{'refId':'A','expr':expr}],
      'fieldConfig':{'defaults':{'unit':unit,'custom':{'spanNulls':False}},'overrides':[]}})
panels.append({'id':4,'title':'Synthetic events (resource samples are not logs)','type':'logs',
    'gridPos':{'x':0,'y':8,'w':24,'h':10},'datasource':{'type':'loki','uid':'loki'},
    'targets':[{'refId':'A','expr':'{job="fixture"}'}],
    'options':{'showTime':True,'showLabels':False,'wrapLogMessage':True}})
(grafana/'dashboards'/'probe.json').write_text(json.dumps({'uid':'synthetic-probe','title':'Local synthetic monitoring probe',
 'schemaVersion':39,'version':1,'editable':False,'time':{'from':'now-5m','to':'now'},'refresh':'5s','panels':panels}))
print('Prepared separate role keys and local data sources; no credentials printed')