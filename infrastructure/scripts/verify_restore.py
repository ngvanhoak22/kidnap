"""Local-only smoke + persistence + restore test; cleans up its own test records."""
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.request
import uuid

if os.environ.get("APP_ENV") != "development":
    raise SystemExit("Restore check only runs with APP_ENV=development.")

root = Path(__file__).resolve().parents[2]
compose = ["docker", "compose", "--env-file", ".env", "-f", "infrastructure/compose.yaml"]
base = "http://127.0.0.1:" + os.environ.get("API_PORT", "8080")
db_user = os.environ.get("POSTGRES_USER", "journey")
source_db = os.environ.get("POSTGRES_DB", "journey")
restore_db = "restore_" + uuid.uuid4().hex + "_test"
created_users = []
restore_created = False
backup = None


def run(args, **kwargs):
    result = subprocess.run(args, cwd=root, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kwargs)
    if result.returncode:
        raise RuntimeError("Command failed: " + args[0] + " (check database/service state)")
    return result.stdout


def sql(database, statement):
    return run(compose + ["exec", "-T", "db", "psql", "-v", "ON_ERROR_STOP=1", "-At", "-U", db_user, "-d", database, "-c", statement])


def request(client, method, path, payload=None, csrf=None):
    body = json.dumps(payload).encode() if payload is not None else None
    headers = {"Content-Type": "application/json"}
    if csrf:
        headers["X-CSRF-Token"] = csrf
    req = urllib.request.Request(base + path, data=body, headers=headers, method=method)
    with client.open(req, timeout=10) as response:
        raw = response.read()
        return json.loads(raw)["data"] if raw else None


def account():
    client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
    email = "restore-" + uuid.uuid4().hex + "@example.com"
    session = request(client, "POST", "/api/v1/auth/register", {
        "email": email, "password": secrets.token_urlsafe(24), "display_name": "Kiểm thử khôi phục"
    })
    # Validate before including an ID in the local verification SQL.
    created_users.append(str(uuid.UUID(session["user"]["id"])))
    return client, email, session["csrf_token"]


def wait_ready():
    for _ in range(80):
        try:
            with urllib.request.urlopen(base + "/health/ready", timeout=2) as response:
                if response.status == 200:
                    return
        except (OSError, TimeoutError):
            pass
        time.sleep(0.25)
    raise RuntimeError("API did not become ready")


try:
    wait_ready()
    owner, _, owner_csrf = account()
    recipient, email, recipient_csrf = account()
    invitation = request(owner, "POST", "/api/v1/trusted-contacts/invitations", {"email": email}, owner_csrf)
    contact = request(recipient, "POST", "/api/v1/trusted-contacts/invitations/" + invitation["invitation"]["id"] + "/accept", {"token": invitation["invitation_token"]}, recipient_csrf)
    contact_id = str(uuid.UUID(contact["id"]))
    request(owner, "PATCH", "/api/v1/trusted-contacts/" + contact_id, {"label": "Người thân thử nghiệm"}, owner_csrf)
    query = "SELECT id,owner_id,contact_user_id,label FROM trusted_contacts WHERE id='" + contact_id + "'"
    before = sql(source_db, query)
    if not before.strip():
        raise RuntimeError("Live API did not persist the accepted contact")

    # Restart only this project's services; unrelated containers are untouched.
    run(compose + ["restart", "db", "api"])
    wait_ready()
    if request(owner, "GET", "/api/v1/auth/session")["user"]["id"] != created_users[0]:
        raise RuntimeError("Session did not survive restart")
    if sql(source_db, query) != before:
        raise RuntimeError("Contact did not survive restart")

    output = run(["sh", "infrastructure/scripts/backup.sh"]).decode().strip()
    backup = root / output.removeprefix("Backup created: ")
    run(compose + ["exec", "-T", "db", "createdb", "-U", db_user, restore_db])
    restore_created = True
    sql(restore_db, "CREATE EXTENSION IF NOT EXISTS timescaledb; CREATE EXTENSION IF NOT EXISTS postgis;")
    sql(restore_db, "SELECT timescaledb_pre_restore();")
    with backup.open("rb") as stream:
        run(compose + ["exec", "-T", "db", "pg_restore", "--exit-on-error", "--no-owner", "-U", db_user, "-d", restore_db], stdin=stream)
    sql(restore_db, "SELECT timescaledb_post_restore();")
    if sql(restore_db, query) != before:
        raise RuntimeError("Restored contact differs from source")
    for user_id in created_users:
        query_user = "SELECT id,email,password_hash FROM users WHERE id='" + user_id + "'"
        if sql(source_db, query_user) != sql(restore_db, query_user):
            raise RuntimeError("Restored user differs from source")
    print("PASS: live API auth/contact flow, restart persistence, pg_dump and isolated restore.")
finally:
    if restore_created:
        run(compose + ["exec", "-T", "db", "dropdb", "-U", db_user, restore_db])
    if created_users:
        ids = ",".join("'" + value + "'" for value in created_users)
        sql(source_db, "DELETE FROM users WHERE id IN (" + ids + ")")
    if backup is not None:
        backup.unlink(missing_ok=True)
