scripts/release.sh builds `winusb-helper-arm64.exe` (from `cmd/winusb-helper`)
into this directory; the Windows x64 build embeds it to install the driver on
ARM64 Windows. Without it the app still builds, but driver installation on
ARM64 Windows reports that the helper is missing.
