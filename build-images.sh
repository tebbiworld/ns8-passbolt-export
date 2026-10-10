#!/bin/bash

#
# Copyright (C) 2026 tebbi
# SPDX-License-Identifier: GPL-3.0-or-later
#

set -e

images=()
repobase="${REPOBASE:-ghcr.io/tebbiworld}"
reponame="passbolt-export"

# Export engine: pbexport (Go, app/) and the official rclone binary. Its base
# images, rclone and the Go modules are pinned in app/Containerfile and
# app/go.mod; the auto-release workflow bumps rclone and go-passbolt there.
app_image="${repobase}/${reponame}-app:${APPTAG:-${IMAGETAG:-latest}}"

echo "Build the export engine image..."
podman build --force-rm -t "${app_image}" -f app/Containerfile app/
podman tag "${app_image}" "${repobase}/${reponame}-app"
images+=("${repobase}/${reponame}-app")

container=$(buildah from scratch)

# Reuse existing nodebuilder-passbolt-export container, to speed up builds
if ! buildah containers --format "{{.ContainerName}}" | grep -q nodebuilder-passbolt-export; then
    echo "Pulling NodeJS runtime..."
    buildah from --name nodebuilder-passbolt-export -v "${PWD}:/usr/src:Z" docker.io/library/node:24.16.0-slim
fi

echo "Build static UI files with node..."
buildah run \
    --workingdir=/usr/src/ui \
    --env="NODE_OPTIONS=--openssl-legacy-provider" \
    nodebuilder-passbolt-export \
    sh -c "yarn install --frozen-lockfile && yarn build"

buildah add "${container}" imageroot /imageroot
buildah add "${container}" ui/dist /ui
# No port, no route, no volume: the engine runs only during an export, in a
# short-lived rootless container. The reader role on Passbolt modules lets the
# settings page list the Passbolt instances of the cluster (get-configuration).
buildah config --entrypoint=/ \
    --label="org.nethserver.rootfull=0" \
    --label="org.nethserver.authorizations=passbolt@any:reader" \
    --label="org.nethserver.images=${app_image}" \
    "${container}"
buildah commit "${container}" "${repobase}/${reponame}"

images+=("${repobase}/${reponame}")

if [[ -n "${CI}" ]]; then
    printf "images=%s\n" "${images[*],,}" >> "${GITHUB_OUTPUT}"
else
    printf "Publish the images with:\n\n"
    printf "  buildah push %s docker://%s\n" "${app_image,,}" "${app_image,,}"
    printf "  buildah push %s docker://%s:%s\n" "${repobase,,}/${reponame,,}" "${repobase,,}/${reponame,,}" "${IMAGETAG:-latest}"
    printf "\n"
fi
