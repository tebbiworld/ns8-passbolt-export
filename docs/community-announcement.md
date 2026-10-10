Title: NS8 Passbolt Export (testing)
Category: App · Tags: ns8

Hello everyone,

I would like to show a new community module and ask for testers: **Passbolt Export** puts a regular, encrypted copy of Passbolt outside the server, as a KeePass file that opens anywhere without a server.

## The problem: a circle

Many of us keep every password in Passbolt, including the ones we would need to bring Passbolt back:

1. Passbolt runs on NethServer 8. Lose the node, its disk or the whole cluster, and Passbolt is gone with it.
2. To restore NethServer 8 you need the login of the backup destination, the encryption password of the backup repository and an admin account. They are in Passbolt.
3. Even a restored Passbolt still needs your recovery kit and passphrase before you can read anything.
4. Without NethServer 8 no Passbolt, without Passbolt no restore of NethServer 8. The module backup of Passbolt does not help here: it can only be restored on a running NethServer 8.

The module breaks the circle. You need only a small offline kit (paper or USB stick in a safe): the login of the upload target, the KeePass password, the key file, and your Passbolt recovery kit for the way back.

## What it does

- Exports what a dedicated, read-only Passbolt account can decrypt, with the folder tree, TOTP data and custom fields, into a KDBX 4 file (Argon2d, ChaCha20).
- Password plus an optional key file. The key file is generated in the browser, downloaded once and never uploaded.
- Uploads with rclone to Dropbox, OneDrive, Google Drive, pCloud, S3, WebDAV or SMB and keeps the last copies (default 12).
- Opens and counts every export again before the upload; checksum and result in a history and in a report mail through the cluster smarthost.
- Daily, weekly or monthly schedule, "Export now", connection check, pinning of the Passbolt server key.
- Passbolt instances of the cluster can be picked from a list.
- An emergency sheet for the safe: after re-entering your cluster admin password, one A4 page with everything needed to open the copy (KeePass password, key file to type in, export account) and empty lines for the cloud login and the NS8 backup key, printed straight from the browser or downloaded as an AES-256 encrypted PDF. The content leaves the server only encrypted with your password.
- Rootless, no route, no port. The export runs in a short-lived container: a small Go program on the official Passbolt SDK (go-passbolt) and rclone. Secrets go to it on stdin only.
- UI in English, German, Italian and French, with an emergency guide.

## Install

Install "Passbolt Export" from the Software Center, or:

```
add-module ghcr.io/tebbiworld/passbolt-export:1.0.0 1
```

If you do not have the repository yet:

```
api-cli run add-repository --data '{"name":"tebbiworld","url":"https://raw.githubusercontent.com/tebbiworld/ns8-repo/main/ns8/updates/","status":true,"testing":false}'
```

## Limits

- Two-factor authentication of the export account works with TOTP only (no security keys).
- Personal items outside shared folders are not exported; share them through a folder.
- The module has to keep the key of the export account to work unattended. Keep that account read-only.

## What I would like to know

- Does the export open in your KeePass app (KeePassXC, Strongbox, KeePassDX, KeeWeb)?
- Which upload target do you use, and did the token setup with `rclone authorize` work for you?
- Is something missing in the emergency guide?

Code and documentation: https://github.com/tebbiworld/ns8-passbolt-export
Wiki: (link follows)
