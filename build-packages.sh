#!/bin/sh
set -eu

transfer_source_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
transfer_build_dir="$transfer_source_dir/build"
transfer_dist_dir="$transfer_source_dir/dist"
transfer_stage_dir="$(mktemp -d)"
trap 'rm -rf "$transfer_stage_dir"' EXIT HUP INT TERM

cd "$transfer_source_dir"
mkdir -p "$transfer_build_dir/方序传文件.app/Contents/MacOS" \
  "$transfer_build_dir/方序传文件.app/Contents/Resources" "$transfer_dist_dir" \
  "$transfer_stage_dir/方序传文件-Windows"

echo "Building Windows x86-64…"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=-s \
  -o "$transfer_stage_dir/方序传文件-Windows/Fangxu-File-Transfer.exe" "$transfer_source_dir"
cp "$transfer_source_dir/packaging/windows/双击运行.bat" "$transfer_stage_dir/方序传文件-Windows/双击运行.bat"
cp "$transfer_source_dir/README.md" "$transfer_stage_dir/方序传文件-Windows/README.md"
cp "$transfer_source_dir/README_EN.md" "$transfer_stage_dir/方序传文件-Windows/README_EN.md"
cp "$transfer_source_dir/LICENSE" "$transfer_stage_dir/方序传文件-Windows/LICENSE"

echo "Building Linux x86-64 and ARM64…"
for transfer_arch in amd64 arm64; do
  transfer_linux_package="$transfer_stage_dir/Fangxu-File-Transfer-Linux-$transfer_arch"
  mkdir -p "$transfer_linux_package"
  CGO_ENABLED=0 GOOS=linux GOARCH="$transfer_arch" go build -trimpath -ldflags=-s \
    -o "$transfer_linux_package/fangxu-file-transfer" "$transfer_source_dir"
  cp "$transfer_source_dir/packaging/linux/双击运行.sh" "$transfer_linux_package/双击运行.sh"
  cp "$transfer_source_dir/README.md" "$transfer_linux_package/README.md"
  cp "$transfer_source_dir/README_EN.md" "$transfer_linux_package/README_EN.md"
  cp "$transfer_source_dir/LICENSE" "$transfer_linux_package/LICENSE"
  chmod +x "$transfer_linux_package/fangxu-file-transfer" "$transfer_linux_package/双击运行.sh"
  tar -C "$transfer_stage_dir" -czf "$transfer_dist_dir/Fangxu-File-Transfer-Linux-$transfer_arch.tar.gz" \
    "Fangxu-File-Transfer-Linux-$transfer_arch"
done

echo "Building universal macOS app…"
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags=-s \
  -o "$transfer_build_dir/fangxu-file-transfer-macos-amd64" "$transfer_source_dir"
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags=-s \
  -o "$transfer_build_dir/fangxu-file-transfer-macos-arm64" "$transfer_source_dir"
lipo -create -output "$transfer_build_dir/方序传文件.app/Contents/MacOS/fangxu-file-transfer-bin" \
  "$transfer_build_dir/fangxu-file-transfer-macos-amd64" "$transfer_build_dir/fangxu-file-transfer-macos-arm64"
cp "$transfer_source_dir/packaging/macos/Info.plist" "$transfer_build_dir/方序传文件.app/Contents/Info.plist"
cp "$transfer_source_dir/packaging/macos/方序传文件" "$transfer_build_dir/方序传文件.app/Contents/MacOS/方序传文件"
transfer_iconset="$transfer_stage_dir/FangxuFileTransfer.iconset"
mkdir -p "$transfer_iconset"
for transfer_size in 16 32 128 256 512; do
  sips -z "$transfer_size" "$transfer_size" "$transfer_source_dir/assets/fangxu-file-transfer-logo.png" \
    --out "$transfer_iconset/icon_${transfer_size}x${transfer_size}.png" >/dev/null
  transfer_retina_size=$((transfer_size * 2))
  sips -z "$transfer_retina_size" "$transfer_retina_size" "$transfer_source_dir/assets/fangxu-file-transfer-logo.png" \
    --out "$transfer_iconset/icon_${transfer_size}x${transfer_size}@2x.png" >/dev/null
done
go run "$transfer_source_dir/packaging/macos/make-icns.go" "$transfer_iconset" \
  "$transfer_build_dir/方序传文件.app/Contents/Resources/FangxuFileTransfer.icns"
chmod +x "$transfer_build_dir/方序传文件.app/Contents/MacOS/方序传文件" \
  "$transfer_build_dir/方序传文件.app/Contents/MacOS/fangxu-file-transfer-bin"
codesign --force --deep --sign - "$transfer_build_dir/方序传文件.app"
ditto -c -k --sequesterRsrc --keepParent "$transfer_build_dir/方序传文件.app" \
  "$transfer_dist_dir/Fangxu-File-Transfer-macOS.zip"

ditto -c -k --norsrc --keepParent "$transfer_stage_dir/方序传文件-Windows" \
  "$transfer_dist_dir/Fangxu-File-Transfer-Windows-amd64.zip"

echo "Packages written to: $transfer_dist_dir"
