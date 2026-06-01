#!/bin/bash

# ==================================================
# Condui Multi Platform Builder
# ==================================================

APP="Condui"

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_DIR="$ROOT_DIR/ssh-gui"

BUILD_RELEASE_DIR="$APP_DIR/build/releases"
DIST_DIR="$ROOT_DIR/dist"


mkdir -p "$DIST_DIR"

cd "$APP_DIR"


# limpiar releases anteriores
rm -rf "$BUILD_RELEASE_DIR"

mkdir -p \
"$BUILD_RELEASE_DIR/windows" \
"$BUILD_RELEASE_DIR/linux" \
"$BUILD_RELEASE_DIR/macos"


build_target () {

    NAME=$1
    PLATFORM=$2
    SAVE_CMD=$3

    echo ""
    echo "================================"
    echo " Building $NAME"
    echo "================================"


    if wails build -clean -platform "$PLATFORM"; then

        echo "Saving artifact..."

        eval "$SAVE_CMD"

        echo "✅ $NAME done"

    else

        echo "❌ $NAME failed"

    fi
}



# ==================================================
# Windows
# ==================================================

build_target \
"Windows x64" \
"windows/amd64" \
"cp build/bin/*.exe '$BUILD_RELEASE_DIR/windows/${APP}-windows-x64.exe'"



# ==================================================
# Linux x64
# ==================================================

build_target \
"Linux x64" \
"linux/amd64" \
"cp build/bin/$APP '$BUILD_RELEASE_DIR/linux/${APP}-linux-x64'"



# ==================================================
# Linux ARM64
# ==================================================

build_target \
"Linux ARM64" \
"linux/arm64" \
"cp build/bin/$APP '$BUILD_RELEASE_DIR/linux/${APP}-linux-arm64'"



# ==================================================
# macOS Intel
# ==================================================

build_target \
"macOS Intel" \
"darwin/amd64" \
"cp -R build/bin/*.app '$BUILD_RELEASE_DIR/macos/${APP}-mac-intel.app'"



# ==================================================
# macOS Apple Silicon
# ==================================================

build_target \
"macOS ARM64" \
"darwin/arm64" \
"cp -R build/bin/*.app '$BUILD_RELEASE_DIR/macos/${APP}-mac-arm64.app'"



# ==================================================
# copiar todo al dist global
# ==================================================

echo ""
echo "Copying final artifacts..."

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

cp -R "$BUILD_RELEASE_DIR/"* "$DIST_DIR"



echo ""
echo "================================"
echo "Generated:"
echo "================================"

find "$DIST_DIR" -maxdepth 3 -type f -o -type d