"""Authenticated bounded observation of synthetic backends; no page scraping."""
import base64
import datetime
import json
import pathlib
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
root=pathlib.Path('/certs'); reports=pathlib.Path('/reports')
context=ssl.create_default_context(cafile=str(root/'ca.crt'))
context.load_cert_chain(str(root/'client.crt'),str(root/'client.key'))
anonymous=ssl.create_default_context(cafile=str(root/'ca.crt'))
credentials=json.loads((root/'grafana-login.json').read_text())
authorization='Basic '+base64.b64encode((credentials['user']+':'+credentials['password']).encode()).decode()
def request(url, *, auth=False, client=True, payload=None):
    headers={'Authorization':authorization} if auth else {}
    encoded = None if payload is None else json.dumps(payload).encode()
    if payload is not None: headers['Content-Type'] = 'application/json'
    with urllib.request.urlopen(urllib.request.Request(url,headers=headers,data=encoded),context=context if client else anonymous,timeout=3) as response:
        body=response.read(512*1024+1)
        if len(body)>512*1024: raise RuntimeError('Response exceeded observation budget')
        return response.status,body.decode()
def observe(name,url,**kwargs):
    status,body=request(url,**kwargs)
    try: data=json.loads(body)
    except ValueError: data=body
    (reports/(name+'.json')).write_text(json.dumps({'at':time.time(),'status':status,'data':data}))
    return data
if sys.argv[1]=='ready':
    deadline=time.monotonic()+75
    endpoints=('https://prometheus:9090/-/ready','https://alertmanager:9093/-/ready','https://loki:3100/ready','http://grafana:3000/api/health')
    remaining=set(endpoints)
    while remaining and time.monotonic()<deadline:
        for url in list(remaining):
            try:
                request(url)
                remaining.remove(url)
            except (OSError,urllib.error.URLError): pass
        if remaining: time.sleep(1)
    if remaining: raise RuntimeError('Backend readiness not observed before deadline: '+','.join(sorted(remaining)))
    print('Backend readiness endpoints answered; no product readiness claim')
elif sys.argv[1]=='observe':
    observations = {}
    for name,url in [('prometheus','https://prometheus:9090/api/v1/query?query='+urllib.parse.quote('diagnostic_fixture_queue_items')),
                     ('source','https://prometheus:9090/api/v1/query?query='+urllib.parse.quote('up{job="fixture"}')),
                     ('rules','https://prometheus:9090/api/v1/alerts'),
                     ('alertmanager','https://alertmanager:9093/api/v2/alerts'),
                     ('logs','https://loki:3100/loki/api/v1/query_range?query='+urllib.parse.quote('{job="fixture"}')+'&limit=100')]:
        observations[name] = observe(name,url)
    observe('grafana-dashboard','http://grafana:3000/api/dashboards/uid/synthetic-probe',auth=True)
    observe('grafana-metrics-health','http://grafana:3000/api/datasources/uid/prometheus/health',auth=True)
    observe('grafana-logs-health','http://grafana:3000/api/datasources/uid/loki/health',auth=True)
    phase = sys.argv[2]
    if phase not in ('normal', 'pressure'):
        raise RuntimeError('Unknown observation phase')
    for name in ('prometheus', 'source', 'rules', 'logs'):
        if observations[name]['status'] != 'success':
            raise RuntimeError(name + ' backend reported failure')
    queue = observations['prometheus']['data']['result']
    source = observations['source']['data']['result']
    expected_queue = 0 if phase == 'normal' else 20
    if len(queue) != 1 or float(queue[0]['value'][1]) != expected_queue:
        raise RuntimeError('Expected synthetic queue value not observed')
    if len(source) != 1 or float(source[0]['value'][1]) != 1:
        raise RuntimeError('Selected fixture scrape was not available')
    records = [json.loads(entry[1]) for stream in observations['logs']['data']['result']
               for entry in stream['values']]
    if not records or not all(record.get('scope') == 'synthetic_fixture' for record in records):
        raise RuntimeError('No scoped synthetic log delivery observed')
    if phase == 'pressure':
        if not any(record.get('event') == 'pressure' for record in records):
            raise RuntimeError('Pressure log event missing')
        if not any(alert['labels']['alertname'] == 'FixtureQueuePressure' and alert['state'] == 'firing'
                   for alert in observations['rules']['data']['alerts']):
            raise RuntimeError('Prometheus pressure alert did not fire')
        if not any(alert['labels']['alertname'] == 'FixtureQueuePressure' and alert['status']['state'] == 'active'
                   for alert in observations['alertmanager']):
            raise RuntimeError('Alertmanager did not receive active pressure alert')
    (reports/'content-assertions.json').write_text(json.dumps(
        {'phase': phase, 'queue_items': expected_queue, 'log_records': len(records), 'passed': True}))
    denials=[]
    for url in ('https://prometheus:9090/-/ready','https://alertmanager:9093/-/ready','https://loki:3100/ready'):
        try: request(url,client=False)
        except (OSError,urllib.error.URLError): denials.append(url)
        else: raise RuntimeError('Unauthenticated backend access succeeded')
    try: request('http://grafana:3000/api/dashboards/uid/synthetic-probe')
    except urllib.error.HTTPError as exc:
        if exc.code!=401: raise
    else: raise RuntimeError('Grafana anonymous dashboard access succeeded')
    (reports/'unauthenticated-refusals.json').write_text(json.dumps({'denied':denials,'grafana_status':401}))
    print('Authenticated observations saved; anonymous accesses refused')
elif sys.argv[1] == 'lifecycle':
    phase = sys.argv[2]
    modes = {
        'silenced': ('FixtureQueuePressure', 'firing', 'suppressed', 1, 1),
        'silence-expired': ('FixtureQueuePressure', 'firing', 'active', 1, 1),
        'threshold-recovered': ('FixtureQueuePressure', None, None, 1, 1),
        'source-pending': ('FixtureObservationUnavailable', 'pending', None, 0, 1),
        'source-firing': ('FixtureObservationUnavailable', 'firing', 'active', 0, 1),
        'source-recovered': ('FixtureObservationUnavailable', None, None, 1, 1),
        'collector-pending': ('CollectorObservationUnavailable', 'pending', None, 1, 0),
        'collector-firing': ('CollectorObservationUnavailable', 'firing', 'active', 1, 0),
        'collector-recovered': ('CollectorObservationUnavailable', None, None, 1, 1),
    }
    if phase not in modes: raise RuntimeError('Unknown lifecycle phase')
    alert_name, rule_state, manager_state, fixture_up, collector_up = modes[phase]
    if phase == 'silenced':
        now = datetime.datetime.now(datetime.timezone.utc)
        silence = observe('created-silence', 'https://alertmanager:9093/api/v2/silences', payload={
            'matchers': [{'name': 'alertname', 'value': alert_name, 'isRegex': False, 'isEqual': True},
                         {'name': 'scope', 'value': 'synthetic_fixture', 'isRegex': False, 'isEqual': True}],
            'startsAt': now.isoformat(), 'endsAt': (now + datetime.timedelta(seconds=12)).isoformat(),
            'createdBy': 'R171 synthetic probe', 'comment': 'Finite synthetic pressure silence; no notifications'})
        if not silence.get('silenceID'): raise RuntimeError('Silence creation did not return identity')
    deadline = time.monotonic() + 25
    attempt = 0
    while time.monotonic() < deadline:
        attempt += 1
        # Preserve each observed transition; a timeout never becomes a passing retry.
        prefix = f'{attempt:02d}-'
        rules = observe(prefix+'rules', 'https://prometheus:9090/api/v1/alerts')['data']['alerts']
        manager = observe(prefix+'manager', 'https://alertmanager:9093/api/v2/alerts')
        up = observe(prefix+'availability', 'https://prometheus:9090/api/v1/query?query='+urllib.parse.quote('up{job=~"fixture|collector"}'))['data']['result']
        queue = observe(prefix+'queue', 'https://prometheus:9090/api/v1/query?query='+urllib.parse.quote('diagnostic_fixture_queue_items'))['data']['result']
        availability = {item['metric']['job']: float(item['value'][1]) for item in up}
        selected_rules = [item for item in rules if item['labels']['alertname'] == alert_name]
        selected_manager = [item for item in manager if item['labels']['alertname'] == alert_name]
        rule_ok = not selected_rules if rule_state is None else len(selected_rules) == 1 and selected_rules[0]['state'] == rule_state
        manager_ok = not selected_manager if manager_state is None else len(selected_manager) == 1 and selected_manager[0]['status']['state'] == manager_state
        metric_ok = availability == {'fixture': fixture_up, 'collector': collector_up}
        if fixture_up == 0: metric_ok = metric_ok and not queue
        if phase in ('silenced', 'silence-expired'):
            metric_ok = metric_ok and len(queue) == 1 and float(queue[0]['value'][1]) == 20
            silences = observe(prefix+'silences', 'https://alertmanager:9093/api/v2/silences')
            expected = 'active' if phase == 'silenced' else 'expired'
            manager_ok = manager_ok and any(item['createdBy'] == 'R171 synthetic probe' and item['status']['state'] == expected for item in silences)
            if phase == 'silenced':
                manager_ok = manager_ok and bool(selected_manager and selected_manager[0]['status']['silencedBy'])
            else:
                manager_ok = manager_ok and bool(selected_manager and not selected_manager[0]['status']['silencedBy'])
        if metric_ok and rule_ok and manager_ok:
            (reports/'lifecycle-assertions.json').write_text(json.dumps({
                'phase': phase, 'at': time.time(), 'rule_state': rule_state,
                'manager_state': manager_state, 'availability': availability,
                'queue_absent': not queue, 'attempts': attempt, 'passed': True}))
            print('Observed synthetic lifecycle phase: '+phase)
            break
        time.sleep(0.5)
    else:
        raise RuntimeError('Expected lifecycle transition not observed within budget: '+phase)
elif sys.argv[1] == 'catchup':
    # Snapshot producer count before polling. A moving target could hide backlog.
    count = observe('producer-watermark', 'https://prometheus:9090/api/v1/query?query='+urllib.parse.quote('diagnostic_fixture_events_total'))['data']['result']
    if len(count) != 1: raise RuntimeError('Producer watermark unavailable')
    watermark = int(float(count[0]['value'][1]))
    deadline = time.monotonic()+20
    attempt = 0
    while time.monotonic() < deadline:
        attempt += 1
        logs = observe(f'{attempt:02d}-logs', 'https://loki:3100/loki/api/v1/query_range?query='+urllib.parse.quote('{job="fixture"}')+'&limit=1000&direction=forward')
        if logs.get('status') != 'success': raise RuntimeError('Log query failed')
        entries = []
        for stream in logs['data']['result']:
            for stamp,line in stream['values']:
                event = json.loads(line)
                event_ns = int(datetime.datetime.fromisoformat(event['at']).timestamp()*1000000000)
                if abs(int(stamp)-event_ns) > 1000:
                    raise RuntimeError('Loki timestamp does not preserve the source event time')
                entries.append(event)
        sequences = [item['sequence'] for item in entries if item.get('scope') == 'synthetic_fixture']
        if set(range(1,watermark+1)).issubset(sequences):
            (reports/'catchup-assertions.json').write_text(json.dumps({
                'watermark':watermark, 'returned_records':len(entries), 'unique_sequences':len(set(sequences)),
                'duplicate_sequences':len(sequences)-len(set(sequences)), 'event_time_preserved':True, 'attempts':attempt, 'passed':True}))
            print('All synthetic sequences through producer watermark arrived in Loki')
            break
        time.sleep(0.5)
    else: raise RuntimeError('Log delivery did not catch up through captured producer watermark')
elif sys.argv[1] == 'shared-interval':
    end = int(time.time()*1000)
    start = end-180000
    request_body = {'from':str(start), 'to':str(end), 'queries':[
        {'refId':'M','datasource':{'type':'prometheus','uid':'prometheus'},'expr':'diagnostic_fixture_queue_items',
         'range':True,'instant':False,'format':'time_series','intervalMs':2000,'maxDataPoints':256},
        {'refId':'L','datasource':{'type':'loki','uid':'loki'},'expr':'{job="fixture"}',
         'queryType':'range','maxLines':1000,'intervalMs':2000,'maxDataPoints':1000}]}
    (reports/'shared-query-input.json').write_text(json.dumps(request_body))
    response = observe('grafana-shared-query','http://grafana:3000/api/ds/query',auth=True,payload=request_body)
    results = response.get('results',{})
    summary = {}
    for ref in ('M','L'):
        result = results.get(ref,{})
        if result.get('error') or result.get('status',200) != 200 or not result.get('frames'):
            raise RuntimeError('Grafana datasource query failed: '+ref)
        rows = 0
        times = []
        values = []
        for frame in result['frames']:
            columns = frame['data']['values']
            if not columns: continue
            rows += len(columns[0])
            for field,column in zip(frame['schema']['fields'],columns):
                if field['type'] == 'time': times.extend(column)
                if ref == 'M' and field['type'] == 'number': values.extend(column)
        if not rows or not times or any(stamp < start or stamp > end for stamp in times):
            raise RuntimeError('Empty or out-of-interval Grafana frames: '+ref)
        if ref == 'M' and (0 not in values or 20 not in values):
            raise RuntimeError('Grafana metric frames omit measured normal/pressure values')
        gaps = []
        if ref == 'M':
            gaps = [right-left for left,right in zip(sorted(times),sorted(times)[1:]) if right-left > 2000]
            if not gaps and None not in values:
                raise RuntimeError('Metric frames contain no gap for the observed source freeze')
        summary[ref] = {'frames':len(result['frames']),'rows':rows,'first_ms':min(times),'last_ms':max(times),'gaps_ms':gaps}
    (reports/'shared-interval-assertions.json').write_text(json.dumps({'from_ms':start,'to_ms':end,'results':summary,'passed':True}))
    print('Grafana returned metric and log frames for the same explicit time interval')
elif sys.argv[1] in ('restart-before', 'restart-after'):
    before_path = pathlib.Path('/history/restart-before/restart-baseline.json')
    if sys.argv[1] == 'restart-before':
        end = int(time.time())
        start = end-180
        before = None
    else:
        before = json.loads(before_path.read_text())
        start, end = before['start_seconds'], before['end_seconds']
    metrics = observe('historical-metrics','https://prometheus:9090/api/v1/query_range?'+urllib.parse.urlencode(
        {'query':'diagnostic_fixture_queue_items','start':start,'end':end,'step':2}))
    metric_samples = [item for series in metrics['data']['result'] for item in series['values']]
    logs = observe('historical-logs','https://loki:3100/loki/api/v1/query_range?'+urllib.parse.urlencode(
        {'query':'{job="fixture"}','start':start*1000000000,'end':end*1000000000,'limit':1000,'direction':'forward'}))
    log_keys = [[stamp,json.loads(line)['sequence']] for series in logs['data']['result'] for stamp,line in series['values']]
    silences = observe('retained-silences','https://alertmanager:9093/api/v2/silences')
    silence_ids = [item['id'] for item in silences if item['createdBy']=='R171 synthetic probe']
    readings = {}
    for name,expr in [('producer','diagnostic_fixture_events_total'),('collector','sum(loki_source_file_read_lines_total)')]:
        result = observe(name,'https://prometheus:9090/api/v1/query?'+urllib.parse.urlencode({'query':expr}))['data']['result']
        if len(result) != 1: raise RuntimeError('Restart observation counter unavailable: '+name)
        readings[name] = int(float(result[0]['value'][1]))
    dashboard = observe('grafana-dashboard','http://grafana:3000/api/dashboards/uid/synthetic-probe',auth=True)
    if dashboard['dashboard']['uid'] != 'synthetic-probe': raise RuntimeError('Restart dashboard identity mismatch')
    snapshot = {'start_seconds':start,'end_seconds':end,'metric_samples':metric_samples,
                'log_keys':log_keys,'silence_ids':silence_ids,'readings':readings}
    if not metric_samples or not log_keys or not silence_ids:
        raise RuntimeError('Missing historical input for backend restart assertion')
    if before is None:
        (reports/'restart-baseline.json').write_text(json.dumps(snapshot))
        print('Captured fixed historical interval, silence IDs and read-counter baseline')
    else:
        if not set(map(tuple,before['metric_samples'])).issubset(set(map(tuple,metric_samples))):
            raise RuntimeError('Historical metric samples lost during backend restart')
        if not set(map(tuple,before['log_keys'])).issubset(set(map(tuple,log_keys))):
            raise RuntimeError('Historical log records lost during backend restart')
        if not set(before['silence_ids']).issubset(silence_ids):
            raise RuntimeError('Silence state lost during Alertmanager restart')
        produced = readings['producer']-before['readings']['producer']
        if produced < 0 or readings['collector'] >= before['readings']['collector']:
            raise RuntimeError('Producer continuity or collector counter reset not observed')
        # Four lines allow scrape timing; this rejects a complete historical reread,
        # not a claim of transactional exactly-once delivery.
        if readings['collector'] > produced+4:
            raise RuntimeError('Collector reread historical file after restart')
        snapshot.pop('metric_samples'); snapshot.pop('log_keys'); snapshot.pop('silence_ids')
        snapshot.update({'metric_rows_retained':len(metric_samples),'log_rows_retained':len(log_keys),
                         'silences_retained':len(silence_ids),'new_source_events':produced,
                         'collector_new_read_lines':readings['collector'],'passed':True})
        (reports/'restart-assertions.json').write_text(json.dumps(snapshot))
        print('Backend restart retained metric/log/silence history without full-file replay')
elif sys.argv[1] in ('backend-baseline', 'backend-unavailable'):
    def backend_counters(prefix):
        data = observe(prefix+'delivery', 'https://prometheus:9090/api/v1/query?'+urllib.parse.urlencode(
            {'query':'{job="collector",__name__=~"loki_write_.*(retries|dropped).*"}'}))
        if data.get('status') != 'success': raise RuntimeError('Delivery counters query failed')
        retries = drops = 0.0
        retry_series = drop_series = 0
        for item in data['data']['result']:
            name = item['metric']['__name__']
            value = float(item['value'][1])
            if 'retries' in name: retries += value; retry_series += 1
            if 'dropped' in name: drops += value; drop_series += 1
        if not retry_series or not drop_series: raise RuntimeError('Actual retry/drop counters unavailable')
        return {'retries':retries,'drops':drops}
    if sys.argv[1] == 'backend-baseline':
        counters = backend_counters('baseline-')
        (reports/'baseline.json').write_text(json.dumps(counters))
        print('Captured actual log-delivery counter baseline')
    else:
        baseline = json.loads(pathlib.Path('/history/backend-baseline/baseline.json').read_text())
        unavailable = False
        try: request('https://loki:3100/ready')
        except (OSError, urllib.error.URLError): unavailable = True
        if not unavailable: raise RuntimeError('Stopped Loki still answered readiness')
        deadline = time.monotonic()+35
        attempt = 0
        while time.monotonic() < deadline:
            attempt += 1
            counters = backend_counters(f'{attempt:02d}-')
            up = observe(f'{attempt:02d}-source-health','https://prometheus:9090/api/v1/query?'+urllib.parse.urlencode(
                {'query':'up{job=~"fixture|collector"}'}))['data']['result']
            health = {item['metric']['job']:float(item['value'][1]) for item in up}
            if counters['retries'] > baseline['retries'] and health == {'fixture':1,'collector':1}:
                if counters['drops'] != baseline['drops']: raise RuntimeError('Log entries dropped during bounded backend outage')
                (reports/'backend-outage-assertions.json').write_text(json.dumps({
                    'backend_unavailable':True,'availability':health,'retry_increase':counters['retries']-baseline['retries'],
                    'drop_increase':counters['drops']-baseline['drops'],'passed':True}))
                print('Unavailable log backend caused visible retries while producer and collector stayed up')
                break
            time.sleep(1)
        else: raise RuntimeError('No visible delivery failure within bounded backend outage')
elif sys.argv[1] in ('storage-baseline', 'storage-pressure', 'storage-recovered'):
    def storage_counter(prefix):
        response = observe(prefix+'wal','https://prometheus:9090/api/v1/query?'+urllib.parse.urlencode(
            {'query':'loki_ingester_wal_disk_full_failures_total{job="logbackend"}'}))
        items = response['data']['result']
        if response.get('status') != 'success' or len(items) != 1:
            raise RuntimeError('Native WAL storage failure counter unavailable')
        return float(items[0]['value'][1])
    if sys.argv[1] == 'storage-baseline':
        baseline = storage_counter('baseline-')
        (reports/'baseline.json').write_text(json.dumps({'wal_failures':baseline}))
        print('Native storage failure counter baseline captured')
    else:
        baseline = json.loads(pathlib.Path('/history/storage-baseline/baseline.json').read_text())['wal_failures']
        deadline = time.monotonic()+40
        attempt = 0
        while time.monotonic() < deadline:
            attempt += 1
            prefix = f'{attempt:02d}-'
            counter = storage_counter(prefix)
            up = observe(prefix+'availability','https://prometheus:9090/api/v1/query?'+urllib.parse.urlencode(
                {'query':'up{job=~"fixture|collector|logbackend"}'}))['data']['result']
            health = {item['metric']['job']:float(item['value'][1]) for item in up}
            rules = observe(prefix+'rules','https://prometheus:9090/api/v1/alerts')['data']['alerts']
            manager = observe(prefix+'manager','https://alertmanager:9093/api/v2/alerts')
            selected_rules = [item for item in rules if item['labels']['alertname']=='LogStoragePressure']
            selected_manager = [item for item in manager if item['labels']['alertname']=='LogStoragePressure']
            if sys.argv[1] == 'storage-pressure':
                state_ok = (len(selected_rules)==1 and selected_rules[0]['state']=='firing' and
                            len(selected_manager)==1 and selected_manager[0]['status']['state']=='active')
            else:
                state_ok = not selected_rules and not selected_manager
            if counter > baseline and health == {'fixture':1,'collector':1,'logbackend':1} and state_ok:
                (reports/'storage-assertions.json').write_text(json.dumps({'phase':sys.argv[1],
                    'native_wal_failure_increase':counter-baseline,'availability':health,
                    'alert_firing':sys.argv[1]=='storage-pressure','passed':True}))
                print('Native storage signal and alert transition observed: '+sys.argv[1])
                break
            time.sleep(1)
        else: raise RuntimeError('Expected storage signal/alert transition not observed within budget')
elif sys.argv[1] == 'healthy-window':
    summary = json.loads(pathlib.Path('/history/resource-window/summary.json').read_text(encoding='utf-8-sig'))
    start = int(datetime.datetime.fromisoformat(summary['started_utc']).timestamp())+2
    end = int(datetime.datetime.fromisoformat(summary['ended_utc']).timestamp())-2
    availability = observe('scrape-history','https://prometheus:9090/api/v1/query_range?'+urllib.parse.urlencode(
        {'query':'up{job=~"fixture|collector"}','start':start,'end':end,'step':2}))['data']['result']
    if len(availability) != 2 or {series['metric']['job'] for series in availability} != {'fixture','collector'}:
        raise RuntimeError('Healthy-window source set unavailable')
    expected_min = int((end-start)/2)-2
    for series in availability:
        values = series['values']
        if len(values) < expected_min or any(float(item[1]) != 1 for item in values):
            raise RuntimeError('Observed scrape loss during healthy resource window')
        if any(right[0]-left[0] > 2 for left,right in zip(values,values[1:])):
            raise RuntimeError('Missing scrape observations during resource window')
    queue = observe('queue-history','https://prometheus:9090/api/v1/query_range?'+urllib.parse.urlencode(
        {'query':'diagnostic_fixture_queue_items','start':start,'end':end,'step':2}))['data']['result']
    if len(queue) != 1 or len(queue[0]['values']) < expected_min or any(float(item[1]) != 0 for item in queue[0]['values']):
        raise RuntimeError('Resource window did not contain the declared healthy fixture input')
    (reports/'healthy-window-assertions.json').write_text(json.dumps({'start_seconds':start,'end_seconds':end,
        'scrape_sources':['fixture','collector'],'queue_rows':len(queue[0]['values']),'passed':True}))
    print('Full resource window retained healthy scrape history and zero fixture pressure')
else: raise RuntimeError('Unknown explicit query mode')