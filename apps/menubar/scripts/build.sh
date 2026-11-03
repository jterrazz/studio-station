#!/bin/sh
set -eu
root=$(cd "$(dirname "$0")/../../.." && pwd)
package="$root/apps/menubar"
app="$root/.artifacts/menubar/Station.app"
scratch="$root/.artifacts/menubar/build"
swift build -c release --package-path "$package" --scratch-path "$scratch" --product StationMenuBar
bin=$(swift build -c release --package-path "$package" --scratch-path "$scratch" --show-bin-path)
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$bin/StationMenuBar" "$app/Contents/MacOS/StationMenuBar"
cp "$package/resources/Info.plist" "$app/Contents/Info.plist"
version=$(git -C "$root" describe --tags --always --dirty | sed 's/^v//')
plutil -replace CFBundleShortVersionString -string "$version" "$app/Contents/Info.plist"
cp "$root/src/ui/testdata/pages/menubar.error.json" "$app/Contents/Resources/stopped.json"
codesign --force --sign - --timestamp=none "$app"
codesign --verify --strict "$app"
printf '● Built %s (%s, signed ad hoc)\n' ".artifacts/menubar/Station.app" "$version"
