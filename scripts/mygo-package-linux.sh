#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-0.1.0-unsigned}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="${REPO_ROOT}/prototypes/mygo-run"
OUT_DIR="${REPO_ROOT}/release"
STAGE_DIR="$(mktemp -d)"
trap 'rm -rf "${STAGE_DIR}"' EXIT

mkdir -p "${OUT_DIR}"
COMMIT="$(git -C "${REPO_ROOT}" rev-parse --short HEAD 2>/dev/null || echo "dev")"
ENDPOINT="${BETA_FEEDBACK_ENDPOINT:-https://dev.asterlab.party/api/feedback}"
UPLOAD_KEY="${BETA_FEEDBACK_KEY:-}"
VER_FLAGS="-X 'asterlab-mygo-prototype/internal/updater.Version=${VERSION}' -X 'asterlab-mygo-prototype/internal/updater.Commit=${COMMIT}' -X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadEndpoint=${ENDPOINT}' -X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadKey=${UPLOAD_KEY}'"
ICON_PNG="${PROTO_DIR}/assets/icon-256.png"

package_arch() {
  local goarch="$1"
  local label="$2"
  local pkg_name="AsterLab-MyGo-${VERSION}-linux-${label}"
  local work="${STAGE_DIR}/${pkg_name}"
  mkdir -p "${work}"

  pushd "${PROTO_DIR}" >/dev/null
  export CGO_ENABLED=0
  GOOS=linux GOARCH="${goarch}" go build -tags=mygo_noinspector -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${work}/mygo-run" .
  GOOS=linux GOARCH="${goarch}" go build -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${work}/asterlab" ./cmd/asterlab
  popd >/dev/null
  chmod +x "${work}/mygo-run" "${work}/asterlab"

  if [[ -f "${ICON_PNG}" ]]; then
    cp "${ICON_PNG}" "${work}/asterlab.png"
  fi

  cat > "${work}/asterlab.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=AsterLab
Comment=Local-First .asterml Wet-Lab Scientific Workbench
Exec=asterlab-gui %f
Icon=asterlab
Terminal=false
Categories=Science;Education;
MimeType=application/x-asterml;x-scheme-handler/asterlab;
StartupWMClass=AsterLab
EOF

  cat > "${work}/asterlab.xml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<mime-info xmlns="http://www.freedesktop.org/standards/shared-mime-info">
  <mime-type type="application/x-asterml">
    <comment>AsterLab Experiment Document</comment>
    <glob pattern="*.asterml"/>
  </mime-type>
</mime-info>
EOF

  cat > "${work}/install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFIX="${HOME}/.local"
APP_DIR="${PREFIX}/share/asterlab"
BIN_DIR="${PREFIX}/bin"
DESKTOP_DIR="${PREFIX}/share/applications"
ICON_DIR="${PREFIX}/share/icons/hicolor/256x256/apps"
MIME_DIR="${PREFIX}/share/mime/packages"

mkdir -p "${APP_DIR}" "${BIN_DIR}" "${DESKTOP_DIR}" "${ICON_DIR}" "${MIME_DIR}"
install -m 0755 "${HERE}/mygo-run" "${APP_DIR}/mygo-run"
install -m 0755 "${HERE}/asterlab" "${APP_DIR}/asterlab"
if [[ -f "${HERE}/asterlab.png" ]]; then
  install -m 0644 "${HERE}/asterlab.png" "${ICON_DIR}/asterlab.png"
fi
ln -sf "${APP_DIR}/mygo-run" "${BIN_DIR}/asterlab-gui"
ln -sf "${APP_DIR}/asterlab" "${BIN_DIR}/asterlab"
sed "s|^Exec=.*|Exec=${BIN_DIR}/asterlab-gui %f|" "${HERE}/asterlab.desktop" > "${DESKTOP_DIR}/asterlab.desktop"
install -m 0644 "${HERE}/asterlab.xml" "${MIME_DIR}/asterlab.xml"
update-mime-database "${PREFIX}/share/mime" >/dev/null 2>&1 || true
update-desktop-database "${DESKTOP_DIR}" >/dev/null 2>&1 || true
echo "Installed AsterLab to ${APP_DIR}. Launch via application menu or '${BIN_DIR}/asterlab-gui'."
EOF
  chmod +x "${work}/install.sh"

  tar -C "${STAGE_DIR}" -czf "${OUT_DIR}/${pkg_name}.tar.gz" "${pkg_name}"
  echo "Linux tarball: ${OUT_DIR}/${pkg_name}.tar.gz"
}

package_arch amd64 x64
package_arch arm64 arm64

# Smoke-test native host x64 CLI binary
"${STAGE_DIR}/AsterLab-MyGo-${VERSION}-linux-x64/asterlab" tools --json >/dev/null

# Build .deb for amd64 (Ubuntu / Debian / Mint / Deepin)
DEB_VER="${VERSION//[^a-zA-Z0-9.+~-]/}"
DEB_VER="${DEB_VER%%-*}"
if [[ -z "${DEB_VER}" ]]; then DEB_VER="0.1.0"; fi
DEB_ROOT="${STAGE_DIR}/deb-amd64"
mkdir -p \
  "${DEB_ROOT}/DEBIAN" \
  "${DEB_ROOT}/opt/asterlab" \
  "${DEB_ROOT}/usr/local/bin" \
  "${DEB_ROOT}/usr/share/applications" \
  "${DEB_ROOT}/usr/share/icons/hicolor/256x256/apps" \
  "${DEB_ROOT}/usr/share/mime/packages"

SRC_X64="${STAGE_DIR}/AsterLab-MyGo-${VERSION}-linux-x64"
install -m 0755 "${SRC_X64}/mygo-run" "${DEB_ROOT}/opt/asterlab/mygo-run"
install -m 0755 "${SRC_X64}/asterlab" "${DEB_ROOT}/opt/asterlab/asterlab"
ln -sf /opt/asterlab/mygo-run "${DEB_ROOT}/usr/local/bin/asterlab-gui"
ln -sf /opt/asterlab/asterlab "${DEB_ROOT}/usr/local/bin/asterlab"
if [[ -f "${SRC_X64}/asterlab.png" ]]; then
  install -m 0644 "${SRC_X64}/asterlab.png" "${DEB_ROOT}/usr/share/icons/hicolor/256x256/apps/asterlab.png"
fi
sed 's|^Exec=.*|Exec=/opt/asterlab/mygo-run %f|' "${SRC_X64}/asterlab.desktop" > "${DEB_ROOT}/usr/share/applications/asterlab.desktop"
install -m 0644 "${SRC_X64}/asterlab.xml" "${DEB_ROOT}/usr/share/mime/packages/asterlab.xml"

cat > "${DEB_ROOT}/DEBIAN/control" <<EOF
Package: asterlab
Version: ${DEB_VER}
Section: science
Priority: optional
Architecture: amd64
Maintainer: AsterLab <hello@asterlab.party>
Description: Local-first .asterml scientific workbench and Pure-Go astercore engine
 Pure-Go desktop workbench and CLI for wet-lab experimental design, execution,
 and deterministic statistical analysis without database lock-in.
EOF

cat > "${DEB_ROOT}/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
update-mime-database /usr/share/mime >/dev/null 2>&1 || true
update-desktop-database /usr/share/applications >/dev/null 2>&1 || true
EOF
chmod 0755 "${DEB_ROOT}/DEBIAN/postinst"

DEB_NAME="AsterLab-MyGo-${VERSION}-linux-amd64.deb"
DEB_PATH="${OUT_DIR}/${DEB_NAME}"
dpkg-deb --build --root-owner-group "${DEB_ROOT}" "${DEB_PATH}"
echo "Linux deb: ${DEB_PATH}"

SUMS_FILE="${OUT_DIR}/sums-linux.txt"
rm -f "${SUMS_FILE}"
for f in \
  "${OUT_DIR}/AsterLab-MyGo-${VERSION}-linux-x64.tar.gz" \
  "${OUT_DIR}/AsterLab-MyGo-${VERSION}-linux-arm64.tar.gz" \
  "${DEB_PATH}"; do
  h="$(sha256sum "${f}" | awk '{print toupper($1)}')"
  sz="$(stat -c%s "${f}")"
  echo "${h}  $(basename "${f}") (${sz} bytes)" >> "${SUMS_FILE}"
done
