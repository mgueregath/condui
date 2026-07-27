#!/bin/bash

set -e

REPO="mgueregath/condui"

echo "================================="
echo " Condui - GitHub Actions secrets"
echo "================================="
echo ""

command -v gh >/dev/null || {
    echo "gh CLI not found. Run: brew install gh && gh auth login"
    exit 1
}

echo "Path to the exported Developer ID .p12 file:"
read P12_PATH

if [ ! -f "$P12_PATH" ]; then
    echo "File not found: $P12_PATH"
    exit 1
fi

base64 -i "$P12_PATH" | gh secret set MACOS_CERTIFICATE_P12 --repo "$REPO"
echo "MACOS_CERTIFICATE_P12 set"

echo ""
echo "Password used to export the .p12:"
read -s P12_PASSWORD
echo ""
echo "$P12_PASSWORD" | gh secret set MACOS_CERTIFICATE_PASSWORD --repo "$REPO"
echo "MACOS_CERTIFICATE_PASSWORD set"

openssl rand -base64 24 | gh secret set MACOS_KEYCHAIN_PASSWORD --repo "$REPO"
echo "MACOS_KEYCHAIN_PASSWORD set (random, generated locally)"

echo "Developer ID Application: Mirko Gueregat (7579K86Z4V)" | gh secret set MACOS_SIGN_IDENTITY --repo "$REPO"
echo "MACOS_SIGN_IDENTITY set"

echo "mgueregath@gmail.com" | gh secret set APPLE_ID --repo "$REPO"
echo "APPLE_ID set"

echo "7579K86Z4V" | gh secret set APPLE_TEAM_ID --repo "$REPO"
echo "APPLE_TEAM_ID set"

echo ""
echo "New app-specific password (from appleid.apple.com, generated for CI):"
read -s APP_PASSWORD
echo ""
echo "$APP_PASSWORD" | gh secret set APPLE_APP_SPECIFIC_PASSWORD --repo "$REPO"
echo "APPLE_APP_SPECIFIC_PASSWORD set"

echo ""
echo "================================="
echo " Done. Verifying:"
echo "================================="
gh secret list --repo "$REPO"
