#!/bin/bash

set -e


# ==========================================
# Paths
# ==========================================

ROOT_DIR="$(pwd)"
WAILS_DIR="$ROOT_DIR/ssh-gui"

DIST="$ROOT_DIR/dist"
WINDOWS_DIST="$DIST/windows"


# ==========================================
# Configuration
# ==========================================

APP_NAME="Condui"

EXE_NAME="${APP_NAME}.exe"

EXE_PATH="$WAILS_DIR/build/bin/$EXE_NAME"

RELEASE_PATH="$WINDOWS_DIST/$EXE_NAME"



echo ""
echo "================================"
echo " Release ${APP_NAME} Windows"
echo "================================"
echo ""



# ==========================================
# Validate tools
# ==========================================


command -v wails >/dev/null || {
    echo "wails not installed"
    exit 1
}



# ==========================================
# Clean
# ==========================================


echo ""
echo "Cleaning Windows release..."
echo ""


# limpia bin temporal de wails
rm -rf "$WAILS_DIR/build/bin"

# limpia SOLO windows dentro de dist
rm -rf "$WINDOWS_DIST"

mkdir -p "$WINDOWS_DIST"



# ==========================================
# Build Wails
# ==========================================


echo ""
echo "Building Wails Windows x64..."
echo ""


cd "$WAILS_DIR"


wails build \
-clean \
-platform windows/amd64


cd "$ROOT_DIR"



# ==========================================
# Validate executable
# ==========================================


if [ ! -f "$EXE_PATH" ]; then

    echo ""
    echo "ERROR: EXE not found:"
    echo "$EXE_PATH"

    exit 1

fi



# ==========================================
# Copy artifact
# ==========================================


echo ""
echo "Copying Windows artifact..."
echo ""


cp "$EXE_PATH" "$RELEASE_PATH"



# ==========================================
# Summary
# ==========================================


echo ""
echo "================================"
echo " DONE"
echo "================================"

echo ""
echo "Created:"
echo "$RELEASE_PATH"

echo ""

find "$WINDOWS_DIST" -maxdepth 2 -type f

echo ""