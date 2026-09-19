"""Local Docker-only security smoke test; creates and removes its own test accounts."""
import base64
import http.cookiejar
import json
import re
import secrets
import time
import urllib.error
import urllib.request
import urllib.parse

BASE = "http://127.0.0.1:8080"
MAIL = "http://127.0.0.1:8025"


class Client:
    def __init__(self):
        self.jar = http.cookiejar.CookieJar()
        self.http = urllib.request.build_opener(urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(self.jar))

    def call(self, method, path, data=None, headers=None, expected=200):
        headers = dict(headers or {})
        for cookie in self.jar:
            if cookie.name == "glipz_csrf":
                headers["X-CSRF-Token"] = cookie.value
        if isinstance(data, dict):
            data = json.dumps(data).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(path if path.startswith("http") else BASE + path, data=data, headers=headers, method=method)
        try:
            response = self.http.open(req, timeout=15)
        except urllib.error.HTTPError as exc:
            response = exc
        body = response.read()
        if response.status != expected:
            raise AssertionError(f"{method} {path.split('?')[0]}: status {response.status}, expected {expected}; {body[:200]!r}")
        if response.headers.get_content_type() == "application/json":
            return json.loads(body)
        return body


def register():
    client = Client()
    handle = "audit" + secrets.token_hex(6)
    email = handle + "@example.test"
    password = secrets.token_urlsafe(24)
    client.call("POST", "/api/v1/auth/register", {
        "email": email, "handle": handle, "password": password,
        "password_confirm": password, "birth_date": "1990-01-01", "terms_agreed": True,
    }, expected=201)
    token = None
    mail_id = None
    for _ in range(20):
        messages = Client().call("GET", MAIL + "/api/v1/messages")
        for msg in messages.get("messages", []):
            if any(item.get("Address") == email for item in msg.get("To", [])):
                mail_id = msg["ID"]
                detail = Client().call("GET", MAIL + "/api/v1/message/" + mail_id)
                match = re.search(r"token=([A-Za-z0-9_-]+)", detail.get("Text", ""))
                if match:
                    token = match.group(1)
                    break
        if token:
            break
        time.sleep(.25)
    if not token:
        raise AssertionError("verification email not received")
    client.call("POST", "/api/v1/auth/register/verify", {"token": token})
    return client, handle, email, password, mail_id


def check_oauth_revocation(owner, viewer):
    anon = Client()
    redirect = "https://example.test/security-callback"
    app = owner.call("POST", "/api/v1/me/oauth-clients", {"name": "Security regression", "redirect_uris": [redirect]}, expected=201)
    credentials = {"client_id": app["client_id"], "client_secret": app["client_secret"]}
    def token(grant, **extra):
        form = urllib.parse.urlencode(dict(credentials, grant_type=grant, **extra)).encode()
        return anon.call("POST", "/api/v1/oauth/token", form, {"Content-Type": "application/x-www-form-urlencoded"})["access_token"]
    client_token = token("client_credentials")
    # A different user authorizes the application: do not confuse app ownership
    # with the subject of an authorization-code access token.
    authorized = viewer.call("POST", "/api/v1/me/oauth-authorize", {"client_id": app["client_id"], "redirect_uri": redirect, "scope": "posts:read", "state": "security-regression"})
    code = urllib.parse.parse_qs(urllib.parse.urlparse(authorized["redirect_to"]).query)["code"][0]
    user_token = token("authorization_code", code=code, redirect_uri=redirect)
    for raw, path in ((client_token, "/api/v1/me"), (user_token, "/api/v1/posts/bookmarks")):
        anon.call("GET", path, headers={"Authorization": "Bearer " + raw})
    pat = viewer.call("POST", "/api/v1/me/personal-access-tokens", {"label": "security-regression"}, expected=201)
    owner.call("DELETE", "/api/v1/me/oauth-clients/" + app["client_id"])
    for raw, path in ((client_token, "/api/v1/me"), (user_token, "/api/v1/posts/bookmarks")):
        anon.call("GET", path, headers={"Authorization": "Bearer " + raw}, expected=401)
    owner.call("GET", "/api/v1/me")
    viewer.call("GET", "/api/v1/me")
    anon.call("GET", "/api/v1/me", headers={"Authorization": "Bearer " + pat["token"]})
    viewer.call("DELETE", "/api/v1/me/personal-access-tokens/" + pat["token_id"])
    for scope in ("", "all", "following", "recommended"):
        viewer.call("GET", "/api/v1/posts/feed?scope=" + scope)
    for offset in (0, 10):
        viewer.call("GET", "/api/v1/posts/feed?scope=invalid-security-regression&offset=" + str(offset), expected=400)
    print("PASS: OAuth client deletion revokes both grants; cross-user authorization, sessions, PATs and feed scope validation")


def run():
    accounts = []
    anon = Client()
    try:
        owner = register(); accounts.append(owner)
        viewer = register(); accounts.append(viewer)
        a, handle, email, password, _ = owner
        b = viewer[0]
        check_oauth_revocation(a, b)
        png = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")
        boundary = "audit" + secrets.token_hex(12)
        body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="audit.png"\r\nContent-Type: image/png\r\n\r\n'.encode() + png + f"\r\n--{boundary}--\r\n".encode())
        upload = a.call("POST", "/api/v1/media/upload", body, {"Content-Type": "multipart/form-data; boundary=" + boundary})
        media = upload["public_url"]
        a.call("GET", media)
        anon.call("GET", media, expected=404)
        post = a.call("POST", "/api/v1/posts", {"caption": "Security smoke test", "media_type": "image", "object_key": upload["object_key"], "visibility": "public"}, expected=201)
        path = "/api/v1/posts/" + post["id"]
        anon.call("GET", media)
        a.call("PATCH", path, {"caption": "Security smoke test", "visibility": "private"})
        for method in ("GET", "HEAD"):
            anon.call(method, media, headers={"Sec-Fetch-Dest": "image", "Range": "bytes=0-1"}, expected=404)
            b.call(method, media, expected=404)
        a.call("GET", media, headers={"Range": "bytes=0-1"}, expected=206)
        a.call("PATCH", path, {"caption": "Security smoke test", "visibility": "logged_in"})
        anon.call("GET", media, expected=404)
        b.call("GET", media)
        a.call("PATCH", path, {"caption": "Security smoke test", "visibility": "followers"})
        b.call("GET", media, expected=404)
        b.call("POST", "/api/v1/users/by-handle/" + handle + "/follow", {})
        b.call("GET", media)
        b.call("POST", "/api/v1/users/by-handle/" + handle + "/follow", {})
        b.call("GET", media, expected=404)
        a.call("PATCH", path, {"caption": "Security smoke test", "visibility": "public", "view_password": "audit-gate-123", "view_password_scope": 8})
        anon.call("GET", media, expected=404)
        b.call("GET", media, expected=404)
        b.call("POST", path + "/unlock", {"password": "audit-gate-123"})
        b.call("GET", media)
        a.call("PATCH", path, {"caption": "Security smoke test", "visibility": "public", "view_password": "audit-gate-456", "view_password_scope": 8})
        b.call("GET", media, expected=404)
        second = Client()
        second.call("POST", "/api/v1/auth/login", {"email": email, "password": password})
        token = next(c.value for c in a.jar if c.name == "glipz_access")
        a.call("POST", "/api/v1/auth/logout", {})
        anon.call("GET", "/api/v1/me", headers={"Authorization": "Bearer " + token}, expected=401)
        second.call("GET", "/api/v1/me")
        a.call("POST", "/api/v1/auth/login", {"email": email, "password": password})
        print("PASS: email registration, upload ownership, public/private/logged-in/followers media, GET/HEAD/Range, password grant/rotation, logout replay, independent session")
    finally:
        for client, _, email, password, mail_id in accounts:
            try:
                client.call("POST", "/api/v1/auth/login", {"email": email, "password": password})
                client.call("DELETE", "/api/v1/me/account", {"password": password, "confirm": "DELETE"})
                if mail_id:
                    Client().call("DELETE", MAIL + "/api/v1/messages", {"IDs": [mail_id]})
            except Exception as exc:
                print("Test-account cleanup needs attention:", str(exc))


if __name__ == "__main__":
    run()
