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
VER_FLAGS="-X 'asterlab-mygo-prototype/internal/updater.Version=${VERSION}' -X 'asterlab-mygo-prototype/internal/updater.Commit=${COMMIT}'"

echo "Building macOS arm64 + amd64 Universal Pure-Go binaries (${VERSION}, ${COMMIT})..."
pushd "${PROTO_DIR}" >/dev/null
export CGO_ENABLED=0
GOOS=darwin GOARCH=arm64 go build -tags=mygo_noinspector -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${STAGE_DIR}/mygo-run-arm64" .
GOOS=darwin GOARCH=amd64 go build -tags=mygo_noinspector -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${STAGE_DIR}/mygo-run-amd64" .
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${STAGE_DIR}/asterlab-arm64" ./cmd/asterlab
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${STAGE_DIR}/asterlab-amd64" ./cmd/asterlab
popd >/dev/null

APP_DIR="${STAGE_DIR}/AsterLab.app"
mkdir -p "${APP_DIR}/Contents/MacOS" "${APP_DIR}/Contents/Resources"
lipo -create -output "${APP_DIR}/Contents/MacOS/mygo-run" "${STAGE_DIR}/mygo-run-arm64" "${STAGE_DIR}/mygo-run-amd64"
lipo -create -output "${APP_DIR}/Contents/MacOS/asterlab" "${STAGE_DIR}/asterlab-arm64" "${STAGE_DIR}/asterlab-amd64"
chmod +x "${APP_DIR}/Contents/MacOS/mygo-run" "${APP_DIR}/Contents/MacOS/asterlab"

# Generate .icns app icon from 256px PNG when available on macOS.
ICON_PNG="${PROTO_DIR}/assets/icon-256.png"
if [[ -f "${ICON_PNG}" ]] && command -v sips >/dev/null 2>&1 && command -v iconutil >/dev/null 2>&1; then
  ICONSET="${STAGE_DIR}/AppIcon.iconset"
  mkdir -p "${ICONSET}"
  for sz in 16 32 128 256; do
    sips -z "${sz}" "${sz}" "${ICON_PNG}" --out "${ICONSET}/icon_${sz}x${sz}.png" >/dev/null
  done
  cp "${ICONSET}/icon_32x32.png" "${ICONSET}/icon_16x16@2x.png"
  cp "${ICONSET}/icon_256x256.png" "${ICONSET}/icon_128x128@2x.png"
  iconutil -c icns "${ICONSET}" -o "${APP_DIR}/Contents/Resources/AppIcon.icns" || true
fi

CLEAN_SHORT="${VERSION%%-*}"
cat > "${APP_DIR}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>
  <string>AsterLab</string>
  <key>CFBundleDisplayName</key>
  <string>AsterLab</string>
  <key>CFBundleIdentifier</key>
  <string>party.asterlab.desktop</string>
  <key>CFBundleVersion</key>
  <string>${VERSION}</string>
  <key>CFBundleShortVersionString</key>
  <string>${CLEAN_SHORT}</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleExecutable</key>
  <string>mygo-run</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>CFBundleURLTypes</key>
  <array>
    <dict>
      <key>CFBundleURLName</key>
      <string>AsterLab Run Deep Link</string>
      <key>CFBundleURLSchemes</key>
      <array>
        <string>asterlab</string>
      </array>
    </dict>
  </array>
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeName</key>
      <string>AsterLab Experiment Document</string>
      <key>CFBundleTypeRole</key>
      <string>Editor</string>
      <key>CFBundleTypeExtensions</key>
      <array>
        <string>asterml</string>
      </array>
    </dict>
  </array>
</dict>
</plist>
EOF

# Ad-hoc sign locally so arm64 Gatekeeper allows execution after user accepts unnotarized prompt.
if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "${APP_DIR}"
fi

# CLI smoke verification inside bundled .app
"${APP_DIR}/Contents/MacOS/asterlab" tools --json >/dev/null

DMG_NAME="AsterLab-MyGo-${VERSION}-macOS-universal.dmg"
ZIP_NAME="AsterLab-MyGo-${VERSION}-macOS-universal.zip"
DMG_PATH="${OUT_DIR}/${DMG_NAME}"
ZIP_PATH="${OUT_DIR}/${ZIP_NAME}"
rm -f "${DMG_PATH}" "${ZIP_PATH}"

DMG_ROOT="${STAGE_DIR}/dmg-root"
mkdir -p "${DMG_ROOT}"
cp -R "${APP_DIR}" "${DMG_ROOT}/AsterLab.app"
ln -s /Applications "${DMG_ROOT}/Applications"

hdiutil create -volname "AsterLab ${VERSION}" -srcfolder "${DMG_ROOT}" -ov -format UDZO "${DMG_PATH}"
ditto -c -k --sequesterRsrc --keepParent "${APP_DIR}" "${ZIP_PATH}"

SUMS_FILE="${OUT_DIR}/sums-macos.txt"
rm -f "${SUMS_FILE}"
for f in "${DMG_PATH}" "${ZIP_PATH}"; do
  h="$(shasum -a 256 "${f}" | awk '{print toupper($1)}')"
  sz="$(stat -f%z "${f}")"
  echo "${h}  $(basename "${f}") (${sz} bytes)" >> "${SUMS_FILE}"
done

echo "macOS DMG: ${DMG_PATH}"
echo "macOS ZIP: ${ZIP_PATH}"
