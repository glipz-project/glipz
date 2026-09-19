"""Create a private, instance-specific .env without touching an existing one."""
import argparse
import base64
import ipaddress
import os
from pathlib import Path
import re
import secrets

ROOT = Path(__file__).resolve().parent.parent


def domain_name(raw):
    domain = raw.strip().lower().encode("idna").decode("ascii")
    if len(domain) > 253 or "." not in domain or any(
        not re.fullmatch(r"[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?", part)
        for part in domain.split(".")
    ):
        raise ValueError("Use a domain name without a scheme, port, path or wildcard")
    try:
        ipaddress.ip_address(domain)
    except ValueError:
        return domain
    raise ValueError("Use a DNS domain, not an IP address")


def email_address(raw):
    if not re.fullmatch(r"[A-Za-z0-9.!_+%-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}", raw):
        raise ValueError("Use a valid operator email address")
    return raw


def generate(domain, email, output):
    domain, email = domain_name(domain), email_address(email)
    values = {
        "GLIPZ_DOMAIN": domain,
        "ACME_EMAIL": email,
        "SMTP_FROM_EMAIL": "no-reply@" + domain,
        "POSTGRES_ADMIN_PASSWORD": secrets.token_hex(32),
        "POSTGRES_APP_PASSWORD": secrets.token_hex(32),
        "REDIS_PASSWORD": secrets.token_hex(32),
        "JWT_SECRET": base64.b64encode(secrets.token_bytes(48)).decode(),
        "GLIPZ_FEDERATION_KEY_SEED": base64.b64encode(secrets.token_bytes(32)).decode(),
    }
    template = (ROOT / ".env.example").read_text(encoding="utf-8")
    for key, value in values.items():
        template = re.sub(r"^" + key + r"=.*$", key + "=" + value, template, flags=re.MULTILINE)
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(template)
    (ROOT / "data" / "legal-docs").mkdir(parents=True, exist_ok=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--domain", required=True)
    parser.add_argument("--email", required=True)
    parser.add_argument("--output", type=Path, default=ROOT / ".env")
    args = parser.parse_args()
    try:
        generate(args.domain, args.email, args.output)
    except (OSError, ValueError) as exc:
        parser.exit(1, f"Setup failed: {exc}\n")
    print("Created private configuration. Set SMTP credentials, prepare legal documents, then run scripts/check-production.py. Existing configuration is never overwritten.")
