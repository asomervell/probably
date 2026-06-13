# Build Assets

This directory contains build assets for the Probably desktop application.

## Required Files

### `appicon.png`
The application icon. Create a 1024x1024 PNG image and place it here.

For macOS, you can generate the icns file using:
```bash
# Install iconutil if needed (comes with Xcode)
mkdir -p appicon.iconset
sips -z 16 16 appicon.png --out appicon.iconset/icon_16x16.png
sips -z 32 32 appicon.png --out appicon.iconset/icon_16x16@2x.png
sips -z 32 32 appicon.png --out appicon.iconset/icon_32x32.png
sips -z 64 64 appicon.png --out appicon.iconset/icon_32x32@2x.png
sips -z 128 128 appicon.png --out appicon.iconset/icon_128x128.png
sips -z 256 256 appicon.png --out appicon.iconset/icon_128x128@2x.png
sips -z 256 256 appicon.png --out appicon.iconset/icon_256x256.png
sips -z 512 512 appicon.png --out appicon.iconset/icon_256x256@2x.png
sips -z 512 512 appicon.png --out appicon.iconset/icon_512x512.png
sips -z 1024 1024 appicon.png --out appicon.iconset/icon_512x512@2x.png
iconutil -c icns appicon.iconset -o darwin/appicon.icns
rm -rf appicon.iconset
```

## Directory Structure

```
build/
├── appicon.png          # Main app icon (1024x1024)
├── darwin/              # macOS-specific files
│   ├── Info.plist       # macOS app metadata
│   └── appicon.icns     # macOS icon (generated)
└── windows/             # Windows-specific files
    ├── info.json        # Windows app metadata
    └── icon.ico         # Windows icon
```

