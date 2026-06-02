#!/bin/bash

set -e


# ==========================================
# Paths
# ==========================================

ROOT_DIR="$(pwd)"
WAILS_DIR="$ROOT_DIR/ssh-gui"

DIST="$ROOT_DIR/dist"


# ==========================================
# Load configuration
# ==========================================

if [ ! -f "$ROOT_DIR/.env" ]; then
    echo "Missing .env"
    echo "Run scripts/setup-macos-signing.sh first"
    exit 1
fi


source "$ROOT_DIR/.env"


APP_PATH="$WAILS_DIR/build/bin/${APP_NAME}.app"

ZIP="$ROOT_DIR/${APP_NAME}.zip"

DMG_NAME="${APP_NAME}.dmg"

DMG_PATH="$DIST/$DMG_NAME"

ENTITLEMENTS="$ROOT_DIR/scripts/entitlements.plist"



echo ""
echo "================================"
echo " Release ${APP_NAME}"
echo "================================"
echo ""

echo "Signing:"
echo "$SIGN_IDENTITY"

echo ""



# ==========================================
# Validate tools
# ==========================================


command -v wails >/dev/null || {
    echo "wails not installed"
    exit 1
}


command -v create-dmg >/dev/null || {

    echo "create-dmg not installed"
    echo ""
    echo "Install:"
    echo "brew install create-dmg"

    exit 1
}



# ==========================================
# Clean
# ==========================================


echo ""
echo "Cleaning..."
echo ""


rm -rf "$WAILS_DIR/build/bin"
rm -rf "$DIST"
rm -f "$ZIP"

mkdir -p "$DIST"



# ==========================================
# Build Wails
# ==========================================


echo ""
echo "Building Wails universal..."
echo ""


cd "$WAILS_DIR"


wails build \
-platform darwin/universal


cd "$ROOT_DIR"



if [ ! -d "$APP_PATH" ]; then

    echo "ERROR: App not found:"
    echo "$APP_PATH"

    exit 1

fi



# ==========================================
# Codesign APP
# ==========================================


echo ""
echo "Signing APP..."
echo ""


codesign \
--force \
--deep \
--options runtime \
--timestamp \
--entitlements "$ENTITLEMENTS" \
--sign "$SIGN_IDENTITY" \
"$APP_PATH"




# ==========================================
# Verify APP
# ==========================================


echo ""
echo "Verify signature..."
echo ""


codesign \
--verify \
--verbose \
--deep \
--strict \
"$APP_PATH"



spctl \
-a \
-vvv \
--type execute \
"$APP_PATH" || true




# ==========================================
# ZIP for notarization
# ==========================================


echo ""
echo "Creating notarization ZIP..."
echo ""


ditto \
-c \
-k \
--keepParent \
"$APP_PATH" \
"$ZIP"




# ==========================================
# Apple Notarization
# ==========================================


echo ""
echo "Sending to Apple..."
echo ""


xcrun notarytool submit "$ZIP" \
--keychain-profile "$NOTARY_PROFILE" \
--wait




# ==========================================
# Staple APP
# ==========================================


echo ""
echo "Stapling..."
echo ""


xcrun stapler staple "$APP_PATH"


xcrun stapler validate "$APP_PATH"





# ==========================================
# Create DMG installer style
# ==========================================


echo ""
echo "Creating DMG..."
echo ""


DMG_WORK="$ROOT_DIR/dmg-work"


rm -rf "$DMG_WORK"
rm -f "$DMG_PATH"


mkdir -p "$DMG_WORK"


echo "Copying APP to DMG workspace..."

cp -R "$APP_PATH" "$DMG_WORK/"


create-dmg \
  --volname "$APP_NAME Installer" \
  --window-pos 200 120 \
  --window-size 600 400 \
  --icon-size 110 \
  --icon "$APP_NAME.app" 150 190 \
  --app-drop-link 430 190 \
  "$DMG_PATH" \
  "$DMG_WORK"



rm -rf "$DMG_WORK"


if [ ! -f "$DMG_PATH" ]; then

    echo "DMG creation failed"
    exit 1

fi


echo ""
echo "Generated DMG:"
echo "$DMG_PATH"


# ==========================================
# Sign DMG
# ==========================================


echo ""
echo "Signing DMG..."
echo ""


codesign \
--force \
--timestamp \
--sign "$SIGN_IDENTITY" \
"$DMG_PATH"

echo ""
echo "Notarizing DMG..."
echo ""


xcrun notarytool submit "$DMG_PATH" \
--keychain-profile "$NOTARY_PROFILE" \
--wait


echo ""
echo "Stapling DMG..."
echo ""


xcrun stapler staple "$DMG_PATH"


xcrun stapler validate "$DMG_PATH"

echo ""
echo "Checking Gatekeeper..."
echo ""


spctl \
-a \
-vvv \
--type install \
"$DMG_PATH"


echo ""
echo "Checking APP inside..."
echo ""


spctl \
-a \
-vvv \
--type execute \
"$APP_PATH"



# ==========================================
# Verify DMG
# ==========================================


echo ""
echo "Verify DMG..."
echo ""


spctl \
-a \
-vvv \
--type install \
"$DMG_PATH"



echo ""
echo "================================"
echo " DONE"
echo "================================"

echo ""
echo "Created:"
echo "$DMG_PATH"

echo ""