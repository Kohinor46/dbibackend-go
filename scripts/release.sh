#!/bin/sh
# Builds release artifacts on macOS:
#   dist/DBI Backend.app      universal (arm64 + x86_64), macOS 12+
#   dist/DBI Backend.exe      Windows x64
# libusb is built from source and linked statically, so neither needs it installed.
#
# Requires: Xcode CLT, Go, pkg-config, fyne CLI (go install fyne.io/tools/cmd/fyne@latest),
#           mingw-w64 (brew install mingw-w64) for the Windows build.
set -eu

LIBUSB_VERSION=${LIBUSB_VERSION:-1.0.30}
APP_NAME="DBI Backend"
APP_ID=com.dbibackend.gui
APP_VERSION=${APP_VERSION:-1.0.1}
MACOS_MIN=12.0

ROOT=$(cd "$(dirname "$0")/.." && pwd)
WORK=$ROOT/.build
DIST=$ROOT/dist
mkdir -p "$WORK" "$DIST"
cd "$WORK"

# pkg-config wrapper so cgo also gets libusb's private (static) link flags.
printf '#!/bin/sh\nexec pkg-config --static "$@"\n' > pkg-config-static
chmod +x pkg-config-static
PKGCFG=$WORK/pkg-config-static

if [ ! -d "libusb-$LIBUSB_VERSION" ]; then
	curl -fsSL -o libusb.tar.bz2 \
		"https://github.com/libusb/libusb/releases/download/v$LIBUSB_VERSION/libusb-$LIBUSB_VERSION.tar.bz2"
	tar xjf libusb.tar.bz2
fi

# build_libusb <name> <configure args...>
build_libusb() {
	name=$1; shift
	[ -f "$WORK/$name/lib/libusb-1.0.a" ] && return
	rm -rf "obj-$name" && mkdir "obj-$name"
	(cd "obj-$name" && "../libusb-$LIBUSB_VERSION/configure" --prefix="$WORK/$name" \
		--disable-shared --enable-static "$@" >/dev/null && make -j8 >/dev/null 2>&1 && make install >/dev/null 2>&1)
}

echo "==> macOS"
build_libusb mac-arm64 --host=aarch64-apple-darwin CC="clang -arch arm64 -mmacosx-version-min=$MACOS_MIN"
build_libusb mac-x86_64 --host=x86_64-apple-darwin CC="clang -arch x86_64 -mmacosx-version-min=$MACOS_MIN"
for arch in arm64 amd64; do
	larch=$([ $arch = arm64 ] && echo arm64 || echo x86_64)
	(cd "$ROOT" && env CGO_ENABLED=1 GOOS=darwin GOARCH=$arch CC="clang -arch $larch" \
		MACOSX_DEPLOYMENT_TARGET=$MACOS_MIN \
		CGO_CFLAGS="-mmacosx-version-min=$MACOS_MIN" CGO_LDFLAGS="-mmacosx-version-min=$MACOS_MIN" \
		PKG_CONFIG="$PKGCFG" PKG_CONFIG_LIBDIR="$WORK/mac-$larch/lib/pkgconfig" \
		go build -trimpath -ldflags="-s -w" -o "$WORK/dbibackend-darwin-$arch" .)
done
lipo -create "$WORK/dbibackend-darwin-arm64" "$WORK/dbibackend-darwin-amd64" -output "$WORK/dbibackend"
rm -rf "$DIST/$APP_NAME.app"
(cd "$ROOT" && fyne package --os darwin --exe "$WORK/dbibackend" --name "$APP_NAME" --app-id $APP_ID \
	--icon "$ROOT/Icon.png" --app-version "$APP_VERSION" --release)
mv "$ROOT/$APP_NAME.app" "$DIST/"
plutil -replace LSMinimumSystemVersion -string $MACOS_MIN "$DIST/$APP_NAME.app/Contents/Info.plist"
codesign --force --deep -s - "$DIST/$APP_NAME.app"

if command -v x86_64-w64-mingw32-gcc >/dev/null; then
	echo "==> Windows"
	build_libusb win64 --host=x86_64-w64-mingw32
	# Native ARM64 driver-install helper embedded into the x64 exe (see winusb/).
	(cd "$ROOT" && GOOS=windows GOARCH=arm64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" -o winusb/helper/winusb-helper-arm64.exe ./cmd/winusb-helper)
	(cd "$ROOT" && env CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
		PKG_CONFIG="$PKGCFG" PKG_CONFIG_LIBDIR="$WORK/win64/lib/pkgconfig" \
		fyne package --os windows --name "$APP_NAME" --app-id $APP_ID \
		--icon "$ROOT/Icon.png" --app-version "$APP_VERSION" --release)
	mv "$ROOT/$APP_NAME.exe" "$DIST/"
else
	echo "==> Windows skipped: mingw-w64 not found (brew install mingw-w64)"
fi

echo "==> Done"
ls -la "$DIST"
