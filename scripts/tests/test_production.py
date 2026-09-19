import base64
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def module(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / "scripts" / (name + ".py"))
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


setup = module("setup-production")
check = module("check-production")


class ProductionConfigTests(unittest.TestCase):
    def test_generator_rejects_injection_and_overwrite(self):
        for domain in ("http://example.com", "example.com/path", "example.com:443", "*.example.com", "127.0.0.1", "example.com\nEVIL=true"):
            with self.assertRaises(ValueError):
                setup.domain_name(domain)
        with self.assertRaises(ValueError):
            setup.email_address("admin@example.com\nEVIL=true")
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "instance.env"
            setup.generate("social.example.com", "admin@example.com", target)
            before = target.read_bytes()
            with self.assertRaises(FileExistsError):
                setup.generate("other.example.com", "admin@example.com", target)
            self.assertEqual(before, target.read_bytes())
            values = dict(line.split("=", 1) for line in before.decode().splitlines() if line and not line.startswith("#"))
            self.assertEqual(32, len(base64.b64decode(values["GLIPZ_FEDERATION_KEY_SEED"])))
            self.assertEqual(3, len({values[k] for k in ("POSTGRES_ADMIN_PASSWORD", "POSTGRES_APP_PASSWORD", "REDIS_PASSWORD")}))

    def test_rendered_production_and_rejected_unsafe_variants(self):
        # Compose renders but does not start any services or request certificates.
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "instance.env"
            setup.generate("social.example.com", "admin@example.com", target)
            target.write_text(target.read_text().replace("SMTP_HOST=\n", "SMTP_HOST=smtp.example.com\n"), encoding="utf-8")
            config = check.load_config(target)
        check.validate(config)
        backend = config["services"]["backend"]
        self.assertTrue(backend["read_only"])
        self.assertNotIn("POSTGRES_ADMIN_PASSWORD", backend["environment"])
        self.assertEqual("https://social.example.com", backend["environment"]["FRONTEND_ORIGIN"])
        self.assertEqual({80, 443}, {int(p["published"]) for p in config["services"]["caddy"]["ports"]})
        for service in ("backend", "postgres", "redis"):
            bad = copy.deepcopy(config)
            bad["services"][service]["ports"] = [{"published": "5432", "target": 5432}]
            with self.assertRaises(ValueError):
                check.validate(bad)
        for key, value in (("SMTP_TLS", "none"), ("JWT_SECRET", "weak"), ("GLIPZ_FEDERATION_KEY_SEED", "invalid"),
                           ("GLIPZ_AUTH_RATE_LIMIT_FAIL_CLOSED", "false"), ("GLIPZ_TRUSTED_PROXY_CIDRS", "0.0.0.0/0")):
            bad = copy.deepcopy(config)
            bad["services"]["backend"]["environment"][key] = value
            with self.assertRaises(ValueError):
                check.validate(bad)


if __name__ == "__main__":
    unittest.main()
