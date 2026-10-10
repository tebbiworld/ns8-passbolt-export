#
# Copyright (C) 2026 tebbi
# SPDX-License-Identifier: GPL-3.0-or-later
#

"""Unit tests of imageroot/pypkg/pbx.py and the configure-module schema.
Run: python3 -m unittest discover -s tests/unit"""

import datetime
import hashlib
import json
import os
import sys
import types
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
sys.path.insert(0, os.path.join(ROOT, "imageroot", "pypkg"))
sys.modules.setdefault("agent", types.ModuleType("agent"))  # NS8 agent SDK, not needed here

import pbx  # noqa: E402

SECRET_FIELDS = {"passbolt_private_key", "passbolt_key_password", "passbolt_totp_secret", "kdbx_password",
                 "kdbx_keyfile_secret", "target_token", "s3_access_key", "s3_secret_access_key", "target_password"}


def keyfile(key):
    digest = hashlib.sha256(key).hexdigest()[:8].upper()
    return f'<KeyFile><Meta><Version>2.0</Version></Meta><Key><Data Hash="{digest}">{key.hex().upper()}</Data></Key></KeyFile>'


class TestSchema(unittest.TestCase):
    def test_secret_fields_are_blanked_by_the_core(self):
        """The NS8 agent replaces task input values with XXX only for names
        ending in password, secret, token, key or pass (htask.go isSensitive).
        Every secret field must follow that rule."""
        path = os.path.join(ROOT, "imageroot", "actions", "configure-module", "validate-input.json")
        props = json.load(open(path))["properties"]
        for name in SECRET_FIELDS:
            self.assertIn(name, props)
            self.assertTrue(name.endswith(("password", "secret", "token", "key", "pass")), name)


class TestValidation(unittest.TestCase):
    def test_keyfile(self):
        self.assertTrue(pbx.check_keyfile(keyfile(bytes(range(32)))))
        broken = keyfile(bytes(range(32))).replace('Hash="', 'Hash="0')
        self.assertFalse(pbx.check_keyfile(broken))
        self.assertFalse(pbx.check_keyfile("<KeyFile/>"))

    def test_path(self):
        self.assertEqual(pbx.check_path("/Backups/Passbolt/"), "Backups/Passbolt")
        self.assertIsNone(pbx.check_path("a/../b"))
        self.assertIsNone(pbx.check_path("a;rm -rf"))
        self.assertEqual(pbx.check_path(""), "")

    def test_totp(self):
        self.assertTrue(pbx.TOTP_RE.match(pbx.normalize_totp("jbsw y3dp ehpk 3pxp")))
        self.assertFalse(pbx.TOTP_RE.match(pbx.normalize_totp("123456")))


class TestSchedule(unittest.TestCase):
    now = datetime.datetime(2026, 10, 10, 12, 0)  # a Saturday

    def sched(self, **kw):
        s = {"enabled": True, "frequency": "weekly", "weekday": "Sun", "monthday": 1, "time": "03:30"}
        s.update(kw)
        return s

    def test_on_calendar(self):
        self.assertEqual(pbx.on_calendar(self.sched()), "Sun *-*-* 03:30:00")
        self.assertEqual(pbx.on_calendar(self.sched(frequency="daily")), "*-*-* 03:30:00")
        self.assertEqual(pbx.on_calendar(self.sched(frequency="monthly", monthday=5)), "*-*-05 03:30:00")

    def test_next_run(self):
        nxt = datetime.datetime.fromtimestamp(pbx.next_run(self.sched(), self.now))
        self.assertEqual(nxt, datetime.datetime(2026, 10, 11, 3, 30))
        nxt = datetime.datetime.fromtimestamp(pbx.next_run(self.sched(frequency="daily", time="13:00"), self.now))
        self.assertEqual(nxt, datetime.datetime(2026, 10, 10, 13, 0))
        nxt = datetime.datetime.fromtimestamp(pbx.next_run(self.sched(frequency="monthly", monthday=1), self.now))
        self.assertEqual(nxt, datetime.datetime(2026, 11, 1, 3, 30))
        self.assertIsNone(pbx.next_run(self.sched(enabled=False), self.now))


if __name__ == "__main__":
    unittest.main()
