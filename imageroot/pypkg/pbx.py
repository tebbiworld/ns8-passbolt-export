#
# Copyright (C) 2026 tebbi
# SPDX-License-Identifier: GPL-3.0-or-later
#

"""Shared code of the passbolt-export actions and the scheduled run.

Secrets (private key, passphrase, TOTP secret, server pin, KeePass password
and key file) live base64-encoded in state/passwords.env (0600, see
modsecrets): several of them span lines or may contain any character. The
credentials of the upload target live only in state/rclone/rclone.conf
(0600), because rclone renews OAuth tokens there by itself.

Nothing in here prints a secret. Values travel to the export engine as one
JSON document on its stdin.
"""

import base64
import calendar
import configparser
import contextlib
import datetime
import email.message
import email.utils
import fcntl
import json
import os
import re
import smtplib
import socket
import ssl
import subprocess
import sys
import time

import agent
import modsecrets

ENGINE_TIMEOUT = 25 * 60
HISTORY_LENGTH = 20

SECRET_NAMES = {
    "private_key": "PB_PRIVATE_KEY_B64",
    "passphrase": "PB_PASSPHRASE_B64",
    "totp_secret": "PB_TOTP_SECRET_B64",
    "verify_token": "PB_VERIFY_TOKEN_B64",
    "verify_enc_token": "PB_VERIFY_ENC_TOKEN_B64",
    "kdbx_password": "KDBX_PASSWORD_B64",
    "keyfile": "KDBX_KEYFILE_B64",
}

# Upload targets offered in the UI. "oauth" types take the token printed by
# `rclone authorize <type>` on the administrator's own computer.
TARGET_TYPES = ("dropbox", "onedrive", "drive", "pcloud", "s3", "webdav", "smb")
OAUTH_TYPES = ("dropbox", "onedrive", "drive", "pcloud")
WEEKDAYS = ("Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun")

TIMER = "passbolt-export.timer"


def state_dir():
    return os.environ.get("AGENT_STATE_DIR") or os.getcwd()


def rclone_dir():
    return os.path.join(state_dir(), "rclone")


def rclone_conf():
    return os.path.join(rclone_dir(), "rclone.conf")


def log(level, msg):
    print(level + msg, file=sys.stderr)


# --- secrets ---------------------------------------------------------------

def get_secret(name):
    raw = modsecrets.get(SECRET_NAMES[name])
    if not raw:
        return ""
    return base64.b64decode(raw).decode("utf-8")


def set_secrets(values):
    """values: name -> str; an empty string removes the secret."""
    modsecrets.write({SECRET_NAMES[k]: base64.b64encode(v.encode("utf-8")).decode("ascii") if v else ""
                      for k, v in values.items()})


def has_secret(name):
    return bool(modsecrets.get(SECRET_NAMES[name]))


# --- validation helpers ----------------------------------------------------

KEYFILE_RE = re.compile(r"<Version>2\.0</Version>.*<Data Hash=\"([0-9A-Fa-f]{8})\">([0-9A-Fa-f\s]{64,200})</Data>", re.S)
TOTP_RE = re.compile(r"^[A-Z2-7]{16,128}=*$")
PATH_RE = re.compile(r"^[A-Za-z0-9 ._/-]{0,200}$")


def check_keyfile(text):
    """True for a KeePass 2.0 XML key file with a matching hash."""
    import hashlib
    m = KEYFILE_RE.search(text or "")
    if not m:
        return False
    key = bytes.fromhex(re.sub(r"\s", "", m.group(2)))
    return len(key) == 32 and hashlib.sha256(key).hexdigest()[:8].upper() == m.group(1).upper()


def normalize_totp(secret):
    return re.sub(r"[\s-]", "", secret or "").upper()


def check_path(path):
    path = (path or "").strip().strip("/")
    if not PATH_RE.match(path) or ".." in path.split("/"):
        return None
    return path


# --- rclone configuration --------------------------------------------------

def read_rclone():
    cp = configparser.RawConfigParser()
    try:
        cp.read(rclone_conf())
    except configparser.Error:
        pass
    return cp


def write_rclone(section):
    """Write the single remote "target"; section is a dict of rclone options."""
    os.makedirs(rclone_dir(), mode=0o700, exist_ok=True)
    os.chmod(rclone_dir(), 0o700)
    cp = configparser.RawConfigParser()
    cp["target"] = section
    tmp = rclone_conf() + ".new"
    old_umask = os.umask(0o077)
    try:
        with open(tmp, "w") as fp:
            cp.write(fp)
    finally:
        os.umask(old_umask)
    os.chmod(tmp, 0o600)
    os.replace(tmp, rclone_conf())


def remove_rclone():
    with contextlib.suppress(FileNotFoundError):
        os.remove(rclone_conf())


def obscure(value):
    """rclone stores passwords obscured (not encrypted) in its file."""
    proc = subprocess.run(["podman", "run", "--rm", "-i", "--network=none", "--cap-drop=all",
                           "--security-opt=no-new-privileges", "--entrypoint", "/usr/local/bin/rclone",
                           os.environ["PASSBOLT_EXPORT_APP_IMAGE"], "obscure", "-"],
                          input=value, capture_output=True, text=True, timeout=120)
    if proc.returncode != 0 or not proc.stdout.strip():
        raise RuntimeError("rclone obscure failed")
    return proc.stdout.strip()


def onedrive_drive(token_json):
    """Look up the personal drive of the token's account (rclone config
    would ask for it interactively)."""
    import urllib.request
    token = json.loads(token_json)
    req = urllib.request.Request("https://graph.microsoft.com/v1.0/me/drive",
                                 headers={"Authorization": "Bearer " + token["access_token"]})
    with urllib.request.urlopen(req, timeout=30) as resp:
        drive = json.load(resp)
    return drive["id"], drive["driveType"]


# --- the export engine -----------------------------------------------------

def remote_path():
    path = os.environ.get("TARGET_PATH", "")
    return "target:" + path


def target_configured():
    return os.environ.get("TARGET_TYPE", "") in TARGET_TYPES and os.path.exists(rclone_conf())


def engine_payload(with_target):
    payload = {
        "server_url": os.environ.get("PASSBOLT_URL", ""),
        "ca_pem": base64.b64decode(os.environ.get("PASSBOLT_CA_B64", "")).decode("utf-8"),
        "private_key": get_secret("private_key"),
        "passphrase": get_secret("passphrase"),
        "totp_secret": get_secret("totp_secret"),
        "verify_token": get_secret("verify_token"),
        "verify_enc_token": get_secret("verify_enc_token"),
    }
    if with_target and target_configured():
        payload["remote"] = remote_path()
    return payload


def local_addresses():
    proc = subprocess.run(["ip", "-j", "addr", "show"], capture_output=True, text=True)
    try:
        return {a["local"] for i in json.loads(proc.stdout or "[]") for a in i.get("addr_info", [])}
    except (ValueError, KeyError):
        return set()


def same_node_hosts():
    """--add-host options for servers that run on this node. A rootless
    container (pasta) owns the node addresses itself, so a connection to the
    node's own address would never leave the container; host-gateway reaches
    the node (Traefik) instead."""
    import urllib.parse
    names = {urllib.parse.urlsplit(os.environ.get(v, "")).hostname
             for v in ("PASSBOLT_URL", "TARGET_WEBDAV_URL", "TARGET_S3_ENDPOINT")}
    names.add(os.environ.get("TARGET_SMB_HOST", ""))
    local = local_addresses()
    options = []
    for name in sorted(n for n in names if n):
        try:
            addrs = {i[4][0] for i in socket.getaddrinfo(name, None)}
        except OSError:
            continue
        if addrs & local:
            options += ["--add-host", name + ":host-gateway"]
    return options


def run_engine(command, payload):
    """Run pbexport in a throw-away container; returns its result document."""
    os.makedirs(rclone_dir(), mode=0o700, exist_ok=True)
    cmd = [
        "podman", "run", "--rm", "-i",
        "--name", os.environ.get("MODULE_ID", "passbolt-export") + "-engine",
        "--replace",
        "--log-driver=none",
        "--read-only",
        "--tmpfs", "/work:rw,size=512m,mode=0700,U",
        "--userns=keep-id:uid=65532,gid=65532",
        "--cap-drop=all",
        "--security-opt=no-new-privileges",
        "--pids-limit=256",
        "--memory=1g",
        "--volume", rclone_dir() + ":/rclone:Z",
        *same_node_hosts(),
        os.environ["PASSBOLT_EXPORT_APP_IMAGE"],
        command,
    ]
    try:
        proc = subprocess.run(cmd, input=json.dumps(payload), capture_output=True, text=True,
                              timeout=ENGINE_TIMEOUT)
    except subprocess.TimeoutExpired:
        subprocess.run(["podman", "rm", "-f", "--ignore", cmd[cmd.index("--name") + 1]], capture_output=True)
        return {"ok": False, "error": "the export engine did not finish in time"}
    for line in (proc.stderr or "").splitlines():
        if line.strip():
            print("engine: " + line[:500], file=sys.stderr)
    try:
        result = json.loads((proc.stdout or "").strip().splitlines()[-1])
    except (ValueError, IndexError):
        result = {"ok": False, "error": "the export engine failed (exit code %d)" % proc.returncode}
    if proc.returncode != 0 and result.get("ok"):
        result["ok"] = False
    return result


def store_pin(result):
    """Keep the server pin of a first contact, but only after a successful
    login, so that a mistyped address is never pinned."""
    if result.get("ok") and result.get("new_verify_token") and result.get("new_verify_enc_token"):
        set_secrets({"verify_token": result.pop("new_verify_token"),
                     "verify_enc_token": result.pop("new_verify_enc_token")})
        return True
    result.pop("new_verify_token", None)
    result.pop("new_verify_enc_token", None)
    return False


@contextlib.contextmanager
def run_lock():
    fd = os.open(os.path.join(state_dir(), ".export.lock"), os.O_CREAT | os.O_RDWR, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise RuntimeError("another export or check of this instance is running")
        yield
    finally:
        os.close(fd)


def export(trigger):
    """One complete export run: engine, status, history and mail."""
    started = int(time.time())
    stamp = datetime.datetime.now().strftime("%Y-%m-%d_%H%M%S")
    prefix = os.environ.get("FILE_PREFIX", "passbolt-export")
    record = {"time": started, "trigger": trigger, "ok": False}
    try:
        with run_lock():
            missing = [n for n in ("private_key", "passphrase", "kdbx_password") if not has_secret(n)]
            if missing or not os.environ.get("PASSBOLT_URL"):
                raise RuntimeError("the module is not configured completely (Passbolt account or KeePass password missing)")
            if not target_configured():
                raise RuntimeError("no upload target configured")
            if os.environ.get("KEYFILE_ENABLED") == "true" and not has_secret("keyfile"):
                raise RuntimeError("the key file is enabled but none is stored")
            payload = engine_payload(True)
            payload.update({
                "kdbx_password": get_secret("kdbx_password"),
                "keyfile": get_secret("keyfile") if os.environ.get("KEYFILE_ENABLED") == "true" else "",
                "file_name": f"{prefix}-{stamp}.kdbx",
                "retention": int(os.environ.get("RETENTION", "12")),
            })
            result = run_engine("run", payload)
            store_pin(result)
    except RuntimeError as exc:
        result = {"ok": False, "error": str(exc)}
    record.update({k: result.get(k) for k in ("ok", "error", "resources", "folders", "entries", "file_name", "size",
                                              "sha256", "verified", "uploaded", "pruned", "kept", "seconds")})
    record["skipped"] = len(result.get("skipped") or [])
    record["skipped_ids"] = (result.get("skipped") or [])[:50]
    record["ok"] = bool(result.get("ok"))
    record["mail"] = send_report(record)
    save_record(record)
    return record


# --- status ----------------------------------------------------------------

def history_path():
    return os.path.join(state_dir(), "runs.json")


def load_history():
    try:
        with open(history_path()) as fp:
            data = json.load(fp)
        return data if isinstance(data, list) else []
    except (FileNotFoundError, ValueError):
        return []


def save_record(record):
    runs = ([record] + load_history())[:HISTORY_LENGTH]
    tmp = history_path() + ".new"
    with open(tmp, "w") as fp:
        json.dump(runs, fp)
    os.replace(tmp, history_path())


# --- schedule --------------------------------------------------------------

def schedule():
    return {
        "enabled": os.environ.get("SCHEDULE_ENABLED", "true") == "true",
        "frequency": os.environ.get("SCHEDULE_FREQUENCY", "weekly"),
        "weekday": os.environ.get("SCHEDULE_WEEKDAY", "Sun"),
        "monthday": int(os.environ.get("SCHEDULE_MONTHDAY", "1")),
        "time": os.environ.get("SCHEDULE_TIME", "03:30"),
    }


def on_calendar(s):
    hhmm = s["time"] + ":00"
    if s["frequency"] == "daily":
        return f"*-*-* {hhmm}"
    if s["frequency"] == "monthly":
        return f"*-*-{s['monthday']:02d} {hhmm}"
    return f"{s['weekday']} *-*-* {hhmm}"


def next_run(s, now=None):
    """Next start in local time as epoch seconds, or None when disabled."""
    if not s["enabled"]:
        return None
    now = now or datetime.datetime.now()
    hour, minute = (int(x) for x in s["time"].split(":"))
    candidate = now.replace(hour=hour, minute=minute, second=0, microsecond=0)
    for _ in range(400):
        due = (s["frequency"] == "daily"
               or (s["frequency"] == "weekly" and WEEKDAYS[candidate.weekday()] == s["weekday"])
               or (s["frequency"] == "monthly" and candidate.day == s["monthday"]))
        if due and candidate > now:
            return int(candidate.timestamp())
        candidate += datetime.timedelta(days=1)
    return None


def apply_schedule():
    """Write the timer drop-in and enable or disable the timer."""
    s = schedule()
    unit_dir = os.path.expanduser("~/.config/systemd/user/" + TIMER + ".d")
    os.makedirs(unit_dir, exist_ok=True)
    with open(os.path.join(unit_dir, "schedule.conf"), "w") as fp:
        fp.write("# written by configure-module from the SCHEDULE_* settings\n"
                 f"[Timer]\nOnCalendar=\nOnCalendar={on_calendar(s)}\n")
    subprocess.run(["systemctl", "--user", "daemon-reload"], check=True)
    if s["enabled"] and target_configured():
        subprocess.run(["systemctl", "--user", "enable", "--now", TIMER], check=True)
        subprocess.run(["systemctl", "--user", "restart", TIMER], check=True)
    else:
        subprocess.run(["systemctl", "--user", "disable", "--now", TIMER], check=False)


# --- mail ------------------------------------------------------------------

MAIL_TEXT = {
    "en": {
        "yes": "yes",
        "no": "no",
        "trigger_manual": "button in the UI",
        "trigger_schedule": "schedule",
        "ok_subject": "Passbolt export OK: {entries} entries",
        "fail_subject": "FAILED: Passbolt export",
        "ok_body": "The Passbolt export finished.\n",
        "fail_body": "The Passbolt export FAILED. No new emergency copy was stored.\n\nError: {error}\n",
        "fields": "File: {file_name}\nEntries: {entries} of {resources} resources ({skipped} not decryptable)\n"
                  "Folders: {folders}\nSize: {size} bytes\nSHA-256: {sha256}\nCopies kept on the target: {kept}\n"
                  "Verified by opening it again: {verified}\nDuration: {seconds} s\n",
        "footer": "\nInstance: {module} on {host}\nTrigger: {trigger}\n",
    },
    "de": {
        "yes": "ja",
        "no": "nein",
        "trigger_manual": "Knopf in der Oberfläche",
        "trigger_schedule": "Zeitplan",
        "ok_subject": "Passbolt-Export OK: {entries} Einträge",
        "fail_subject": "FEHLER: Passbolt-Export",
        "ok_body": "Der Passbolt-Export ist abgeschlossen.\n",
        "fail_body": "Der Passbolt-Export ist FEHLGESCHLAGEN. Es wurde keine neue Notfallkopie abgelegt.\n\nFehler: {error}\n",
        "fields": "Datei: {file_name}\nEinträge: {entries} von {resources} Objekten ({skipped} nicht entschlüsselbar)\n"
                  "Ordner: {folders}\nGröße: {size} Byte\nSHA-256: {sha256}\nStände am Ziel: {kept}\n"
                  "Durch erneutes Öffnen geprüft: {verified}\nDauer: {seconds} s\n",
        "footer": "\nInstanz: {module} auf {host}\nAuslöser: {trigger}\n",
    },
    "it": {
        "yes": "sì",
        "no": "no",
        "trigger_manual": "pulsante nell'interfaccia",
        "trigger_schedule": "pianificazione",
        "ok_subject": "Esportazione Passbolt OK: {entries} voci",
        "fail_subject": "ERRORE: esportazione Passbolt",
        "ok_body": "L'esportazione di Passbolt è terminata.\n",
        "fail_body": "L'esportazione di Passbolt è FALLITA. Non è stata salvata una nuova copia di emergenza.\n\nErrore: {error}\n",
        "fields": "File: {file_name}\nVoci: {entries} di {resources} risorse ({skipped} non decifrabili)\n"
                  "Cartelle: {folders}\nDimensione: {size} byte\nSHA-256: {sha256}\nCopie conservate: {kept}\n"
                  "Verificato riaprendolo: {verified}\nDurata: {seconds} s\n",
        "footer": "\nIstanza: {module} su {host}\nAvvio: {trigger}\n",
    },
    "fr": {
        "yes": "oui",
        "no": "non",
        "trigger_manual": "bouton de l'interface",
        "trigger_schedule": "planification",
        "ok_subject": "Export Passbolt OK : {entries} entrées",
        "fail_subject": "ÉCHEC : export Passbolt",
        "ok_body": "L'export Passbolt est terminé.\n",
        "fail_body": "L'export Passbolt a ÉCHOUÉ. Aucune nouvelle copie de secours n'a été enregistrée.\n\nErreur : {error}\n",
        "fields": "Fichier : {file_name}\nEntrées : {entries} sur {resources} ressources ({skipped} non déchiffrables)\n"
                  "Dossiers : {folders}\nTaille : {size} octets\nSHA-256 : {sha256}\nCopies conservées : {kept}\n"
                  "Vérifié en le rouvrant : {verified}\nDurée : {seconds} s\n",
        "footer": "\nInstance : {module} sur {host}\nDéclenchement : {trigger}\n",
    },
}

EMAIL_RE = re.compile(r"^[^@\s<>\"',;]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$")


def send_report(record):
    """Mail the run result through the cluster smarthost. Returns sent,
    skipped or failed; never raises."""
    to_addr = os.environ.get("NOTIFY_EMAIL", "")
    mode = os.environ.get("NOTIFY_MODE", "always")
    if not to_addr or mode == "never" or (mode == "failure" and record["ok"]):
        return "skipped"
    try:
        smarthost = agent.get_smarthost_settings(agent.redis_connect())
        if not smarthost["enabled"] or not smarthost["host"]:
            log(agent.SD_WARNING, "no smarthost configured in the cluster: report mail not sent")
            return "skipped"
        lang = os.environ.get("MAIL_LANGUAGE", "en")
        text = MAIL_TEXT.get(lang, MAIL_TEXT["en"])
        values = {k: ("" if record.get(k) is None else record.get(k)) for k in
                  ("entries", "resources", "skipped", "folders", "size", "sha256", "kept", "seconds", "file_name",
                   "trigger", "verified")}
        values.update({"error": (record.get("error") or "")[:1000], "module": os.environ.get("MODULE_ID", ""),
                       "host": socket.getfqdn(), "trigger": text.get("trigger_" + record["trigger"], record["trigger"]),
                       "verified": text["yes"] if record.get("verified") else text["no"]})
        msg = email.message.EmailMessage()
        if record["ok"]:
            msg["Subject"] = text["ok_subject"].format(**values)
            body = text["ok_body"] + "\n" + text["fields"].format(**values)
        else:
            msg["Subject"] = text["fail_subject"].format(**values)
            body = text["fail_body"].format(**values)
        msg.set_content(body + text["footer"].format(**values))
        sender = os.environ.get("NOTIFY_FROM") or (smarthost["username"] if "@" in smarthost["username"]
                                                   else "no-reply@" + socket.getfqdn())
        msg["From"] = sender
        msg["To"] = to_addr
        msg["Date"] = email.utils.formatdate(localtime=True)
        msg["Message-ID"] = email.utils.make_msgid()
        context = ssl.create_default_context()
        if not smarthost["tls_verify"]:
            context.check_hostname = False
            context.verify_mode = ssl.CERT_NONE
        if smarthost["encrypt_smtp"] == "tls":
            server = smtplib.SMTP_SSL(smarthost["host"], smarthost["port"], timeout=60, context=context)
        else:
            server = smtplib.SMTP(smarthost["host"], smarthost["port"], timeout=60)
            if smarthost["encrypt_smtp"] == "starttls":
                server.starttls(context=context)
        with server:
            if smarthost["username"]:
                server.login(smarthost["username"], smarthost["password"])
            server.send_message(msg)
        return "sent"
    except Exception as exc:  # the export result must be recorded even if mail fails
        log(agent.SD_WARNING, "report mail failed: %s" % type(exc).__name__)
        return "failed"
