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
else: raise RuntimeError('Unknown explicit query mode')