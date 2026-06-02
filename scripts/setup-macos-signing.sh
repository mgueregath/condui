#!/bin/bash

set -e

ENV_FILE=".env"

DEFAULT_PROFILE="condui-profile"


echo "================================="
echo " Condui macOS signing setup"
echo "================================="
echo ""


echo "Apple ID:"
read APPLE_ID


echo "Team ID:"
read TEAM_ID


echo "App specific password:"
read -s APP_PASSWORD

echo ""


echo "Notary profile name [$DEFAULT_PROFILE]:"
read PROFILE

if [ -z "$PROFILE" ]; then
    PROFILE=$DEFAULT_PROFILE
fi


echo ""
echo "Searching Developer ID certificates..."
echo ""


security find-identity -v -p codesigning


echo ""
echo "Copy the EXACT Developer ID Application name:"
echo "Example:"
echo "Developer ID Application: Mirko Gueregat (ABCDE12345)"
echo ""

read SIGN_IDENTITY



echo ""
echo "Saving Apple notarization credentials..."
echo ""


xcrun notarytool store-credentials "$PROFILE" \
  --apple-id "$APPLE_ID" \
  --team-id "$TEAM_ID" \
  --password "$APP_PASSWORD"



echo ""
echo "Creating $ENV_FILE"
echo ""


cat > "$ENV_FILE" <<EOF
# =========================
# Condui macOS Release
# =========================

APP_NAME=Condui

APPLE_ID=$APPLE_ID
APPLE_TEAM_ID=$TEAM_ID

NOTARY_PROFILE=$PROFILE

SIGN_IDENTITY="$SIGN_IDENTITY"

EOF



echo ""
echo "Loading environment..."
echo ""

source "$ENV_FILE"



echo "Validating configuration..."
echo ""


if security find-identity -v -p codesigning | grep -q "$SIGN_IDENTITY"
then
    echo "Certificate OK"
else
    echo "Certificate not found"
    exit 1
fi



echo ""
echo "Setup completed"
echo ""
echo "APP_NAME=$APP_NAME"
echo "PROFILE=$NOTARY_PROFILE"
echo "SIGN=$SIGN_IDENTITY"
echo ""