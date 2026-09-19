"""Validate production Compose configuration without printing resolved secrets."""
import argparse
import base64
import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess

from importlib.util import module_from_spec, spec_from_file_location

ROOT = Path(__file__).resolve().parent.parent
spec = spec_from_file_location("production_setup", ROOT / "scripts" / "setup-production.py")
setup = module_from_spec(spec)
spec.loader.exec_module(setup)


def validate(config):
    services = config["services"]
    backend = services["backend"]["environment"]
    edge = services["caddy"]["environment"]
    domain = setup.domain_name(edge["GLIPZ_DOMAIN"])
    setup.email_address(edge["ACME_EMAIL"])
    setup.email_address(backend["SMTP_FROM_EMAIL"])
    for name in ("backend", "postgres", "redis"):
        if services[name].get("ports"):
            raise ValueError(f"{name} must not publish host ports")
    if "mailpit" in services:
        raise ValueError("Development mail server present in production")
    if "POSTGRES_ADMIN_PASSWORD" in backend or "POSTGRES_PASSWORD" in backend:
        raise ValueError("Database administrator credentials must not reach the application")
    passwords = [services["postgres"]["environment"]["POSTGRES_PASSWORD"],
                 services["postgres"]["environment"]["GLIPZ_APP_PASSWORD"],
                 services["redis"]["environment"]["REDISCLI_AUTH"]]
    if any(not re.fullmatch(r"[0-9a-f]{64}", password) for password in passwords) or len(set(passwords)) != 3:
        raise ValueError("Generate distinct 64-character hex database/Redis passwords")
    if len(backend["JWT_SECRET"]) < 64:
        raise ValueError("JWT_SECRET must contain at least 64 random characters")
    try:
        seed = base64.b64decode(backend["GLIPZ_FEDERATION_KEY_SEED"], validate=True)
    except ValueError:
        raise ValueError("Invalid federation seed encoding") from None
    if len(seed) != 32:
        raise ValueError("Federation seed must decode to 32 bytes")
    for key in ("FRONTEND_ORIGIN", "GLIPZ_PROTOCOL_PUBLIC_ORIGIN"):
        if backend[key] != "https://" + domain:
            raise ValueError(f"{key} must match the HTTPS domain")
    if backend["SMTP_TLS"] not in ("starttls", "tls"):
        raise ValueError("SMTP_TLS must explicitly require starttls or tls")
    if backend["SMTP_HOST"] in ("mailpit", "localhost", "127.0.0.1"):
        raise ValueError("Configure a real external SMTP provider")
    if not 1 <= int(backend["SMTP_PORT"]) <= 65535:
        raise ValueError("Invalid SMTP port")
    for key in ("GLIPZ_AUTH_RATE_LIMIT_FAIL_CLOSED", "GLIPZ_REMOTE_MEDIA_PROXY_RATE_LIMIT_FAIL_CLOSED",
                "GLIPZ_LINK_PREVIEW_RATE_LIMIT_FAIL_CLOSED", "GLIPZ_FEDERATION_INBOX_RATE_LIMIT_FAIL_CLOSED"):
        if str(backend[key]).lower() != "true":
            raise ValueError(f"{key} must be true")
    networks = config["networks"]
    if not all(networks[name].get("internal") for name in ("proxy", "database")):
        raise ValueError("Proxy and database networks must be internal")
    peer = ipaddress.ip_address(services["caddy"]["networks"]["proxy"]["ipv4_address"])
    subnet = ipaddress.ip_network(networks["proxy"]["ipam"]["config"][0]["subnet"])
    backend_peer = ipaddress.ip_address(services["backend"]["networks"]["proxy"]["ipv4_address"])
    if backend_peer not in subnet or backend_peer == peer:
        raise ValueError("Backend proxy address must be distinct and within the proxy subnet")
    if peer not in subnet or str(peer) + "/32" != backend["GLIPZ_TRUSTED_PROXY_CIDRS"]:
        raise ValueError("Proxy address, subnet and trusted peer do not match")
    if backend["GLIPZ_STORAGE_MODE"] == "s3":
        if any(not backend.get(key) for key in ("S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY", "S3_BUCKET")):
            raise ValueError("Complete the S3 configuration")
        if not backend["S3_ENDPOINT"].startswith("https://"):
            raise ValueError("Production S3 endpoint must use HTTPS")
    elif backend["GLIPZ_STORAGE_MODE"] != "local":
        raise ValueError("Unknown storage mode")


def load_config(env_file):
    result = subprocess.run(["docker", "compose", "--env-file", str(env_file), "-f", str(ROOT / "docker-compose.yml"),
                             "config", "--format", "json"], cwd=ROOT, capture_output=True, text=True)
    if result.returncode:
        raise ValueError("Compose configuration is incomplete. Check required values in .env.example (resolved secrets are not printed)")
    return json.loads(result.stdout)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", type=Path, default=ROOT / ".env")
    args = parser.parse_args()
    try:
        if os.name == "posix" and args.env_file.stat().st_mode & 0o077:
            raise ValueError("Set .env permissions to 0600")
        validate(load_config(args.env_file))
    except (OSError, ValueError, KeyError) as exc:
        parser.exit(1, f"Preflight failed: {exc}\n")
    print("PASS: production configuration, private services, dedicated secrets, proxy trust and mail TLS. DNS, SMTP delivery, restore and public HTTPS still require live verification.")
