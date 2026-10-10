#!/usr/bin/env python3
"""Compare the pinned upstream versions with the newest releases.

Two upstreams are tracked:
  rclone       docker.io/rclone/rclone, pinned by tag and digest in
               app/Containerfile (the binary is copied from that image)
  go-passbolt  github.com/passbolt/go-passbolt, the Passbolt SDK in app/go.mod

Writes current / newest / newer to $GITHUB_OUTPUT and the commands that
perform the bump to .upstream_bump.sh (run from the repository root).
"""
import json
import os
import re
import subprocess
import sys
import urllib.request

CONTAINERFILE = "app/Containerfile"
GOMOD = "app/go.mod"
RCLONE = "docker.io/rclone/rclone"
SDK = "github.com/passbolt/go-passbolt"


def skopeo(*args):
    return json.loads(subprocess.run(["skopeo", *args], capture_output=True, text=True, check=True).stdout)


def numeric(tag):
    return [int(x) for x in re.findall(r"\d+", tag)]


def github_tags(repo):
    tags = []
    for page in range(1, 6):
        req = urllib.request.Request(f"https://api.github.com/repos/{repo}/tags?per_page=100&page={page}",
                                     headers={"Accept": "application/vnd.github+json", "User-Agent": "ns8-upstream-check"})
        if os.environ.get("GITHUB_TOKEN"):
            req.add_header("Authorization", "Bearer " + os.environ["GITHUB_TOKEN"])
        batch = json.load(urllib.request.urlopen(req, timeout=30))
        tags += [t["name"] for t in batch]
        if len(batch) < 100:
            break
    return tags


containerfile = open(CONTAINERFILE).read()
m = re.search(rf"{re.escape(RCLONE)}:(\d+\.\d+\.\d+)@(sha256:[0-9a-f]{{64}})", containerfile)
if not m:
    sys.exit("rclone pin not found in " + CONTAINERFILE)
rclone_cur, rclone_digest = m.group(1), m.group(2)
rclone_tags = [t for t in skopeo("list-tags", f"docker://{RCLONE}")["Tags"] if re.fullmatch(r"\d+\.\d+\.\d+", t)]
rclone_new = max(rclone_tags, key=numeric)

m = re.search(rf"^\s*{re.escape(SDK)} (v\d+\.\d+\.\d+)\s*$", open(GOMOD).read(), re.M)
if not m:
    sys.exit("go-passbolt pin not found in " + GOMOD)
sdk_cur = m.group(1)
sdk_new = max((t for t in github_tags("passbolt/go-passbolt") if re.fullmatch(r"v\d+\.\d+\.\d+", t)), key=numeric)

go_image = re.search(r"FROM (docker\.io/library/golang:\S+) AS builder", containerfile).group(1)

bump = ["set -e"]
if numeric(rclone_new) > numeric(rclone_cur):
    digest = skopeo("inspect", f"docker://{RCLONE}:{rclone_new}")["Digest"]
    bump.append(f"sed -i 's|{RCLONE}:{rclone_cur}@{rclone_digest}|{RCLONE}:{rclone_new}@{digest}|' {CONTAINERFILE}")
if numeric(sdk_new) > numeric(sdk_cur):
    bump.append(f"docker run --rm -v \"$PWD/app:/src\" -w /src {go_image} "
                f"sh -c 'go get {SDK}@{sdk_new} && go mod tidy'")
newer = len(bump) > 1

current = f"rclone {rclone_cur}, go-passbolt {sdk_cur}"
newest = f"rclone {rclone_new}, go-passbolt {sdk_new}"
with open(".upstream_bump.sh", "w") as fp:
    fp.write("\n".join(bump) + "\n")
print(f"current: {current}; newest: {newest}; newer={newer}")
with open(os.environ.get("GITHUB_OUTPUT", "/dev/null"), "a") as fp:
    fp.write(f"current={current}\nnewest={newest}\nnewer={'true' if newer else 'false'}\n")
