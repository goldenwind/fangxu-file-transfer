"""Collect native installers and the Windows portable ZIP for GitHub Releases."""
import argparse
import json
import os
from pathlib import Path
import shutil
from zipfile import ZIP_DEFLATED, ZipFile

EXPECTED = {
    "macOS-arm64": {".dmg"},
    "macOS-x64": {".dmg"},
    "Windows-x64": {".exe"},
    "Linux-x64": {".deb", ".AppImage"},
    "Linux-arm64": {".deb"},
}


def collect_installers(version, platform, output):
    expected = EXPECTED[platform]
    found = set()
    for source in sorted(Path("src-tauri/target/release/bundle").glob("*/*")):
        if source.is_file() and source.suffix in expected:
            if source.suffix in found:
                raise RuntimeError(f"Duplicate {source.suffix} installer: {source}")
            found.add(source.suffix)
            destination = output / f"Fangxu-File-Transfer-{version}-{platform}{source.suffix}"
            shutil.copy2(source, destination)
            print(destination)
    if found != expected:
        raise RuntimeError(f"Missing installers for {platform}: {expected - found}")


def collect_windows_portable(version, output):
    # The release client resolves its Go component next to its own executable.
    files = {
        Path("src-tauri/target/release/fangxu-file-transfer-desktop.exe"): "方序传文件.exe",
        Path("src-tauri/binaries/fangxu-transfer-service-x86_64-pc-windows-msvc.exe"):
            "fangxu-transfer-service.exe",
        Path("docs/windows-portable.txt"): "使用说明-README.txt",
        Path("LICENSE"): "LICENSE.txt",
    }
    for source in files:
        if not source.is_file():
            raise RuntimeError(f"Missing Windows portable file: {source}")
    folder = f"Fangxu-File-Transfer-{version}-Windows-x64-portable"
    destination = output / f"{folder}.zip"
    with ZipFile(destination, "w", compression=ZIP_DEFLATED) as archive:
        for source, name in files.items():
            archive.write(source, f"{folder}/{name}")
    print(destination)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", choices=EXPECTED, default=os.environ.get("PACKAGE_PLATFORM"))
    parser.add_argument("--portable-only", action="store_true",
                        help="Package Windows release binaries without requiring an installer")
    args = parser.parse_args()
    if not args.platform:
        parser.error("set PACKAGE_PLATFORM or pass --platform")
    if args.portable_only and args.platform != "Windows-x64":
        parser.error("--portable-only requires --platform Windows-x64")
    version = json.loads(Path("package.json").read_text(encoding="utf-8"))["version"]
    output = Path("release-assets")
    output.mkdir(exist_ok=True)
    if not args.portable_only:
        collect_installers(version, args.platform, output)
    if args.platform == "Windows-x64":
        collect_windows_portable(version, output)


if __name__ == "__main__":
    main()
