"""Create/remove an isolated account for localhost UI regression tests."""
import importlib.util
import json
import pathlib
import sys

root = pathlib.Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('smoke', root / 'scripts/security-smoke.py')
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)
path = root / 'data/ux-browser-fixture.json'
if sys.argv[1] == 'create':
    if path.exists():
        raise SystemExit('UI fixture exists. Run cleanup before creating another.')
    c, handle, email, password, mail = smoke.register()
    path.parent.mkdir(exist_ok=True)
    path.write_text(json.dumps(dict(email=email, password=password, handle=handle, mail=mail)), encoding='utf-8')
    c.call('POST', '/api/v1/posts', dict(caption='UI regression fixture. ' + '読みやすさの検証。' * 12, media_type='text', visibility='public'), expected=201)
    print('UI fixture ready')
elif sys.argv[1] == 'cleanup' and path.exists():
    fixture = json.loads(path.read_text(encoding='utf-8'))
    c = smoke.Client()
    c.call('POST', '/api/v1/auth/login', dict(email=fixture['email'], password=fixture['password']))
    c.call('DELETE', '/api/v1/me/account', dict(password=fixture['password'], confirm='DELETE'))
    smoke.Client().call('DELETE', smoke.MAIL + '/api/v1/messages', {'IDs': [fixture['mail']]})
    path.unlink()
    print('UI fixture removed')
