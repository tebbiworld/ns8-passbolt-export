# ns8-passbolt-export

NethServer 8 module that exports Passbolt on a schedule into an encrypted
KeePass database (KDBX 4) and uploads it to a place outside the server:
Dropbox, Microsoft OneDrive, Google Drive, pCloud, S3 compatible storage,
WebDAV or an SMB share.

The file opens on any computer or phone with KeePassXC, Strongbox, KeePassDX or
KeeWeb, without a server and without Passbolt. It is the emergency copy for the
day Passbolt or the whole cluster is gone.

## Why: the circular dependency

Many installations keep every password in Passbolt, including the ones needed
to bring Passbolt back:

1. Passbolt runs on NethServer 8. If the node, its disk or the cluster is lost,
   Passbolt is gone with it.
2. Restoring NethServer 8 needs the login of the backup destination, the
   encryption password of the backup repository and an administrator account.
   They are in Passbolt.
3. Even a restored Passbolt still needs the recovery kit (private key) and its
   passphrase before anything can be read.
4. So: without NethServer 8 no Passbolt, without Passbolt no restore of
   NethServer 8. The module backup of Passbolt does not break this circle,
   because it can only be restored on a running NethServer 8.

This module breaks it: a current, encrypted copy of the passwords lies outside
the server. To open it you need only an **offline emergency kit**, kept outside
Passbolt and outside the server (paper, USB stick, safe):

- the login of the upload target (including its two-factor recovery codes),
- the KeePass password and, if enabled, the key file,
- your own Passbolt recovery kit and passphrase, for the way back,
- the encryption password of the NS8 backup repository and an admin login.

If one of these exists only in Passbolt, the emergency copy is useless in
exactly the case it is made for.

## Features

- Folder tree of Passbolt kept as KeePass groups; name, user name, URL,
  password, description, TOTP (as `otp` field) and custom fields.
- KDBX 4 with Argon2d (64 MiB, 10 iterations) and ChaCha20; password
  (min. 12 characters) plus an optional key file. The key file is generated in
  the browser, downloaded once and never uploaded to the target.
- Every export is decrypted and counted again before the upload; the SHA-256
  checksum, the number of entries and the result are kept in a history and
  mailed after each run (through the cluster smarthost; de, en, it, fr).
- Schedule daily, weekly or monthly at a chosen time, "Export now" button,
  connection check. The oldest copies beyond a retention count (default 12)
  are deleted; only files written by the module are touched.
- The Passbolt server key is pinned on the first successful login, so a
  replaced server is refused instead of trusted.
- Passbolt instances of the cluster are offered for selection; any other
  Passbolt address works too. A private CA can be added for Passbolt and the
  upload target.
- **Emergency sheet**: after re-entering the password of the logged-in
  cluster administrator, one A4 page with the KeePass password, the key file
  to type in again (grouped hex plus the XML form with its check hash), the
  passphrase and TOTP secret of the export account, and empty lines for the
  login of the upload target and the NS8 backup repository key is printed
  straight from the browser. The recovery kit of the export account can be
  downloaded the same way. The module sends the content only encrypted with
  that password (PBKDF2-SHA256, AES-256-GCM), because NS8 keeps task output in
  Redis for hours; the browser decrypts it, prints it and forgets it (the print
  dialog's "Save as PDF" would not be encrypted). Alternatively the same page
  is downloaded as PDF 2.0 encrypted with AES-256 (revision 6, pdfcpu), open
  password = that cluster password, printing allowed, changes not; it travels
  sealed the same way. Five wrong passwords lock the sheet for 15 minutes.
- UI in English, German, Italian and French, with an emergency guide.

## How it works

The module has no port, no route and no long-running service. A systemd timer
(or the button) starts a short-lived rootless container
`ghcr.io/tebbiworld/passbolt-export-app` with:

- `pbexport`, a small Go program in [`app/`](app/) built on the official
  Passbolt SDK [go-passbolt](https://github.com/passbolt/go-passbolt) and
  [gokeepasslib](https://github.com/tobischo/gokeepasslib). The entry mapping
  follows `export keepass` of
  [go-passbolt-cli](https://github.com/passbolt/go-passbolt-cli) (MIT), which
  writes all entries flat, without folders and without a key file.
- the official [rclone](https://rclone.org) binary for the upload.

All settings including the secrets go to the container as one JSON document on
stdin: nothing sensitive is in the process list, the container definition or
its environment. The clear text never touches a disk; the encrypted file lives
in a tmpfs until it is uploaded. The container runs with a read-only root,
no capabilities, `no-new-privileges` and a memory limit.

The export contains exactly what the configured Passbolt account can decrypt
(end-to-end encryption). Use a dedicated account with read-only access to the
shared folders; see the Guide page.

## Setup

1. In Passbolt create a user for the export (role User), finish its setup and
   keep its recovery kit (`passbolt-recovery-kit.asc`) and passphrase.
2. If two-factor authentication is required, enable **TOTP** for this account
   and note the base32 secret shown below the QR code. Security keys (WebAuthn)
   are not supported.
3. Share your top-level folders with the account, permission "can read". Items
   created later in a shared folder through the Passbolt web interface are
   shared with it as well. Personal items outside shared folders are not
   exported.
4. For Dropbox, OneDrive, Google Drive or pCloud run on your own computer
   `rclone authorize "dropbox"` (or `onedrive`, `drive`, `pcloud`), log in and
   copy the printed token.
5. In the module settings enter the Passbolt address, the recovery kit, the
   passphrase, the TOTP secret, the KeePass password, the key file option and
   the target, then press "Check connection" and "Export now" on the status
   page.

Wiki: https://wiki.nethserver.org/doku.php?id=ns8:applications:passbolt-export

Install from the Software Center (repository `tebbiworld`) or:

```
add-module ghcr.io/tebbiworld/passbolt-export:1.0.0 1
```

## Where things are stored

| What | Where |
|---|---|
| Settings (address, target type and path, schedule, mail) | `state/environment` |
| Private key, passphrase, TOTP secret, server pin, KeePass password, key file | `state/passwords.env` (0600, values base64 encoded) |
| Credentials of the upload target (OAuth token, keys, obscured password) | `state/rclone/rclone.conf` (0600; rclone renews tokens there) |
| Run history | `state/runs.json` |

All four are in the module backup (`imageroot/etc/state-include.conf`) and are
protected by the encryption of the NS8 backup repository. Secret input fields
are named so that the NS8 agent blanks them in the task context it keeps in
Redis (names ending in `password`, `secret`, `token` or `key`).

## Security notes and limits

- Whoever controls the node can read what the export account can read: the
  module must hold its key, passphrase and TOTP secret to work unattended.
  Keep the account read-only and the node patched.
- The copy on the target is as strong as the KeePass password and the key
  file.
- The export is one-way: a reading copy, not a restore path into Passbolt.
  Restore Passbolt itself from the NS8 backup.
- Not exported: comments, tags, sharing permissions, password expiry.
- A clone of the instance exports to the same target with the same settings;
  change the file name prefix or disable the schedule of one of them.

## Testing

- `app/`: `go test ./...` (run in the image build).
- `tests/unit`: `python3 -m unittest discover -s tests/unit`.
- `tests/*.robot`: install, configuration, validation, secret leak check,
  backup and restore, clone, removal (QEMU CI through
  `stephdl/ns8-ci-actions`).

## Logo

The logo combines the Passbolt icon (from
[passbolt_styleguide](https://github.com/passbolt/passbolt_styleguide),
AGPL-3.0; Passbolt is a trademark of Passbolt SA), the KeePassXC icon
(Lorenzo Stella, LGPL-2) and the ISO 7010 E003 first aid sign. This module is
not affiliated with or endorsed by Passbolt SA or the KeePassXC team.

## License

GPL-3.0-or-later, see [LICENSE](LICENSE).
