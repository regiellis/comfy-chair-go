import uuid, threading, time
from aiohttp import web
from .nodes import NODE_CLASS_MAPPINGS

ENGINE = list(NODE_CLASS_MAPPINGS.values())[0]() if NODE_CLASS_MAPPINGS else None
routes = web.RouteTableDef()
BATCH = {}

def json_or_empty(text: str):
    if not text:
        return {}
    import json as _j
    try:
        return _j.loads(text)
    except Exception:
        return {}

async def safe_json(request):
    try:
        return await request.json()
    except Exception:
        return json_or_empty(await request.text())

@routes.post('/{{NodeNameLower}}/generate')
async def generate(request: web.Request):
    body = await safe_json(request)
    topic = body.get('topic')
    if not topic:
        return web.json_response({'success': False, 'error': 'topic required'}, status=400)
    try:
        result = ENGINE.run(topic)[0] if ENGINE else 'no-engine'
        return web.json_response({'success': True, 'result': result})
    except Exception as e:  # pragma: no cover
        return web.json_response({'success': False, 'error': str(e)}, status=500)

@routes.get('/{{NodeNameLower}}/health')
async def health(_):
    ok, err = True, None
    try:
        if not ENGINE:
            raise RuntimeError('engine missing')
    except Exception as e:
        ok, err = False, str(e)
    return web.json_response({'ok': ok, 'error': err})

@routes.get('/{{NodeNameLower}}/version')
async def version(_):
    return web.json_response({'success': True, 'version': '0.1.0', 'name': '{{NodeName}}'})

@routes.post('/{{NodeNameLower}}/batch_start')
async def batch_start(request: web.Request):
    body = await safe_json(request)
    topics = body.get('topics') or []
    job_id = uuid.uuid4().hex
    cancel = threading.Event()
    BATCH[job_id] = {'status': 'running', 'progress': 0, 'results': [], 'cancel': cancel}
    threading.Thread(target=_run_batch, args=(job_id, topics), daemon=True).start()
    return web.json_response({'success': True, 'job_id': job_id})

def _run_batch(job_id, topics):
    job = BATCH[job_id]
    total = max(len(topics), 1)
    for i, t in enumerate(topics or ['demo']):
        if job['cancel'].is_set():
            job['status'] = 'cancelled'
            return
        try:
            res = ENGINE.run(t)[0] if ENGINE else f'no-engine-{t}'
        except Exception as e:  # pragma: no cover
            res = f'error:{e}'
        job['results'].append(res)
        job['progress'] = int(((i + 1) / total) * 100)
        time.sleep(0.05)
    job['status'] = 'completed'

@routes.get('/{{NodeNameLower}}/batch_status')
async def batch_status(request: web.Request):
    job_id = request.query.get('job_id')
    job = BATCH.get(job_id)
    if not job:
        return web.json_response({'success': False, 'error': 'unknown job'}, status=404)
    return web.json_response({'success': True, 'data': {'status': job['status'], 'progress': job['progress']}})

@routes.get('/{{NodeNameLower}}/batch_results')
async def batch_results(request: web.Request):
    job_id = request.query.get('job_id')
    job = BATCH.get(job_id)
    if not job:
        return web.json_response({'success': False, 'error': 'unknown job'}, status=404)
    return web.json_response({'success': True, 'results': job['results'], 'status': job['status']})

@routes.post('/{{NodeNameLower}}/batch_cancel')
async def batch_cancel(request: web.Request):
    body = await safe_json(request)
    job_id = body.get('job_id')
    job = BATCH.get(job_id)
    if not job:
        return web.json_response({'success': False, 'error': 'unknown job'}, status=404)
    job['cancel'].set()
    job['status'] = 'cancelled'
    return web.json_response({'success': True, 'cancelled': True})

def setup_api_routes(app):
    app.add_routes(routes)
    return app
