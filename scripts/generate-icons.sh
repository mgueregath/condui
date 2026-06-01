#!/bin/bash

set -e

# =====================================================
# condui - icon generator
# Source: ssh-gui/build/condui.png
# Outputs:
#   ssh-gui/build/appicon.png
#   ssh-gui/build/darwin/icon.icns
#   ssh-gui/build/windows/icon.ico
#   ssh-gui/build/linux/icon.png
# =====================================================

SRC="ssh-gui/build/condui.png"

if [ ! -f "$SRC" ]; then
    echo "❌ Missing $SRC"
    exit 1
fi


echo "🚀 Generating condui icons..."


# -----------------------------------------------------
# Common Wails icon
# -----------------------------------------------------

cp "$SRC" ssh-gui/build/appicon.png

echo "✓ appicon.png"



# -----------------------------------------------------
# macOS .icns
# -----------------------------------------------------

echo "🍎 macOS icon..."

rm -rf ssh-gui/build/condui.iconset

mkdir -p ssh-gui/build/condui.iconset
mkdir -p ssh-gui/build/darwin


sips -z 16 16 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_16x16.png >/dev/null


sips -z 32 32 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_16x16@2x.png >/dev/null


sips -z 32 32 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_32x32.png >/dev/null


sips -z 64 64 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_32x32@2x.png >/dev/null


sips -z 128 128 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_128x128.png >/dev/null


sips -z 256 256 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_128x128@2x.png >/dev/null


sips -z 256 256 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_256x256.png >/dev/null


sips -z 512 512 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_256x256@2x.png >/dev/null


sips -z 512 512 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_512x512.png >/dev/null


sips -z 1024 1024 \
"$SRC" \
--out ssh-gui/build/condui.iconset/icon_512x512@2x.png >/dev/null



iconutil \
-c icns \
ssh-gui/build/condui.iconset \
-o ssh-gui/build/darwin/icon.icns


rm -rf ssh-gui/build/condui.iconset

echo "✓ ssh-gui/build/darwin/icon.icns"



# -----------------------------------------------------
# Windows .ico
# -----------------------------------------------------

echo "🪟 Windows icon..."

mkdir -p ssh-gui/build/windows


if command -v magick >/dev/null 2>&1; then

    magick "$SRC" \
    -define icon:auto-resize=256,128,64,48,32,16 \
    ssh-gui/build/windows/icon.ico

elif command -v png2ico >/dev/null 2>&1; then

    png2ico \
    ssh-gui/build/windows/icon.ico \
    "$SRC"

else

    echo "⚠️ Install imagemagick or png2ico"
    echo "brew install imagemagick"
    exit 1

fi


echo "✓ ssh-gui/build/windows/icon.ico"



# -----------------------------------------------------
# Linux PNG icons
# -----------------------------------------------------

echo "🐧 Linux icons..."

mkdir -p ssh-gui/build/linux


cp "$SRC" \
ssh-gui/build/linux/icon.png


mkdir -p ssh-gui/build/linux/hicolor/256x256/apps
mkdir -p ssh-gui/build/linux/hicolor/512x512/apps


sips -z 256 256 \
"$SRC" \
--out ssh-gui/build/linux/hicolor/256x256/apps/condui.png >/dev/null


sips -z 512 512 \
"$SRC" \
--out ssh-gui/build/linux/hicolor/512x512/apps/condui.png >/dev/null


echo "✓ Linux icons"



echo ""
echo "================================="
echo " condui icons generated 🎉"
echo "================================="

tree build | head -40