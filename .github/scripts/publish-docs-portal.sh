#!/bin/bash
# Publish the built rspress site (website/doc_build) to documentation.scality.com
# or documentation-internal.scality.com, under OpenSource/<project>/<version>.
# The build is portable, so the same files work under <version>/ and latest/.
set -euo pipefail

: "${DOCS_VERSION:?}" "${DOCUMENTATION_SSH_USER:?}" "${DOCUMENTATION_SITE_PATH:?}"
PROJECT_DIR="${DOCUMENTATION_SITE_PATH}/OpenSource/mountpoint-s3-csi-driver"
DESTINATION="${PROJECT_DIR}/${DOCS_VERSION}"
REMOTE="${DOCUMENTATION_SSH_USER}@documentation"
TARBALL="mountpoint-s3-csi-driver-docs-${DOCS_VERSION}.tar.gz"

tar --create --gzip --file "${TARBALL}" --directory website/doc_build .
rsync "${TARBALL}" "${REMOTE}:/tmp/"
# Replace the version folder so pages removed from the docs disappear too.
ssh "${REMOTE}" sudo rm -rf "${DESTINATION}"
ssh "${REMOTE}" sudo mkdir -p "${DESTINATION}"
ssh "${REMOTE}" sudo tar -xzf "/tmp/${TARBALL}" -C "${DESTINATION}"
ssh "${REMOTE}" rm -f "/tmp/${TARBALL}"
ssh "${REMOTE}" sudo ln -nsf "${DESTINATION}" "${PROJECT_DIR}/latest"

if [[ -n "${DOCUMENTATION_WEBSITE_FILE_OWNER:-}" ]]; then
  ssh "${REMOTE}" sudo chown -R "${DOCUMENTATION_WEBSITE_FILE_OWNER}" "${PROJECT_DIR}"
fi
ssh "${REMOTE}" sudo find "${PROJECT_DIR}" -type f -exec chmod 664 '{}' +
ssh "${REMOTE}" sudo find "${PROJECT_DIR}" -type d -exec chmod 775 '{}' +
ssh "${REMOTE}" sudo restorecon -R "${PROJECT_DIR}"
echo "Published ${DESTINATION}"
