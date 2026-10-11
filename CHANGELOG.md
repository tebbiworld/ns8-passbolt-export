# Changelog

## Unreleased

- The module backup no longer contains `state/passwords.env` (private key,
  passphrase and TOTP secret of the export account, KeePass password, key
  file). After restoring the module from a backup, enter them again in the
  settings; the status page lists what is missing and the export does not run
  until then. Clones are unchanged. Backups taken before this version still
  hold the file until the repository prunes them.

## 1.0.0 — 2026-10-10

- First version: scheduled export of Passbolt into an encrypted KeePass file
  (KDBX 4, password and optional key file) with the Passbolt folder tree,
  uploaded with rclone to Dropbox, OneDrive, Google Drive, pCloud, S3, WebDAV
  or SMB, retention, verification by re-opening, report mail, server key
  pinning, emergency guide. UI in English, German, Italian and French.
