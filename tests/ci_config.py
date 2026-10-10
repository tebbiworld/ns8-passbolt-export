#
# Copyright (C) 2026 tebbi
# SPDX-License-Identifier: GPL-3.0-or-later
#

"""Settings for the CI suites. There is no Passbolt and no upload target in
the CI: the addresses point at the node itself on a closed port, so every
connection is refused at once. All secret values are dummies made for the
test, recognisable in a leak check."""

import json

KEYFILE = ('<?xml version="1.0" encoding="utf-8"?>\n<KeyFile><Meta><Version>2.0</Version></Meta><Key>'
           '<Data Hash="630DCD29">00010203 04050607 08090A0B 0C0D0E0F 10111213 14151617 18191A1B 1C1D1E1F</Data>'
           '</Key></KeyFile>\n')

PRIVATE_KEY = ("-----BEGIN PGP PRIVATE KEY BLOCK-----\n\nci-dummy-private-key-line\n"
               "-----END PGP PRIVATE KEY BLOCK-----\n")

SECRETS = {
    "passbolt_key_password": "ci-passphrase-4711",
    "passbolt_totp_secret": "JBSWY3DPEHPK3PXPJBSWY3DP",
    "kdbx_password": "ci-kdbx-password-0815",
    "target_password": "ci-dav-password-42",
}


def config(**override):
    data = {
        "passbolt_url": "https://passbolt.ci.test:9", "passbolt_ca_pem": "",
        "passbolt_private_key": PRIVATE_KEY, "clear_totp_secret": False,
        "keyfile_enabled": True, "kdbx_keyfile_secret": KEYFILE,
        "target_type": "webdav", "target_path": "Passbolt/CI", "target_token": "", "pcloud_hostname": "",
        "s3_provider": "", "s3_endpoint": "", "s3_region": "", "s3_access_key": "", "s3_secret_access_key": "",
        "webdav_url": "https://dav.ci.test:9/dav", "webdav_vendor": "other", "smb_host": "", "smb_domain": "",
        "target_user": "ci", "file_prefix": "ci-export", "retention": 5,
        "schedule_enabled": True, "schedule_frequency": "monthly", "schedule_weekday": "Sun",
        "schedule_monthday": 15, "schedule_time": "04:15", "notify_email": "", "notify_from": "",
        "notify_mode": "always", "mail_language": "de",
    }
    data.update(SECRETS)
    data.update(override)
    return json.dumps(data)


def keep_secrets(**override):
    """The same settings with every secret field empty (= keep)."""
    empty = {k: "" for k in list(SECRETS) + ["passbolt_private_key", "kdbx_keyfile_secret"]}
    empty.update(override)
    return config(**empty)


def leak_values():
    return list(SECRETS.values()) + ["ci-dummy-private-key-line", "08090A0B"]
