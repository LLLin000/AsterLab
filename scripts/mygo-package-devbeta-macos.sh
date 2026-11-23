#!/usr/bin/env bash
set -euo pipefail

# Builds the explicitly-labeled AsterLab BetaFeedback macOS universal zip (-tags=devdebug)
# for invited beta testers. Public end-user releases remain strictly -tags=mygo_noinspector.
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

VER_FLAGS="-X 'asterlab-mygo-prototype/internal/updater.Version=${VERSION}-beta' -X 'asterlab-mygo-prototype/internal/updater.Commit=${COMMIT}' -X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadEndpoint=${ENDPOINT}' -X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadKey=${UPLOAD_KEY}'"

echo "Building macOS BetaFeedback Universal binary (${VERSION}, ${COMMIT})..."
pushd "${PROTO_DIR}" >/dev/null
export CGO_ENABLED=0
GOOS=darwin GOARCH=arm64 go build -tags="e2e,devdebug" -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${STAGE_DIR}/mygo-run-arm64" .
GOOS=darwin GOARCH=amd64 go build -tags="e2e,devdebug" -trimpath -ldflags="-s -w ${VER_FLAGS}" -o "${STAGE_DIR}/mygo-run-amd64" .
popd >/dev/null

APP_DIR="${STAGE_DIR}/AsterLab-BetaFeedback.app"
mkdir -p "${APP_DIR}/Contents/MacOS" "${APP_DIR}/Contents/Resources"
lipo -create -output "${APP_DIR}/Contents/MacOS/mygo-run" "${STAGE_DIR}/mygo-run-arm64" "${STAGE_DIR}/mygo-run-amd64"
chmod +x "${APP_DIR}/Contents/MacOS/mygo-run"

CLEAN_SHORT="${VERSION%%-*}"
cat > "${APP_DIR}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>AsterLab BetaFeedback</string>
  <key>CFBundleDisplayName</key><string>AsterLab BetaFeedback</string>
  <key>CFBundleIdentifier</key><string>party.asterlab.desktop.beta</string>
  <key>CFBundleVersion</key><string>${VERSION}-beta</string>
  <key>CFBundleShortVersionString</key><string>${CLEAN_SHORT}</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleExecutable</key><string>mygo-run</string>
  <key>LSMinimumSystemVersion</key><string>12.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF

if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "${APP_DIR}" || true
fi

ZIP_PATH="${OUT_DIR}/AsterLab-BetaFeedback-${VERSION}-macOS-universal.zip"
rm -f "${ZIP_PATH}"
ditto -c -k --sequesterRsrc --keepParent "${APP_DIR}" "${ZIP_PATH}"
echo "BetaFeedback macOS zip ready: ${ZIP_PATH}"
