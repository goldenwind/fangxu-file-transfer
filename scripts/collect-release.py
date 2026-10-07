"""Give native installers unique, readable names for GitHub Releases."""
import json
import os
from pathlib import Path
import shutil

version = json.loads(Path("package.json").read_text())["version"]
platform = os.environ["PACKAGE_PLATFORM"]
expected = {
    "macOS-arm64": {".dmg"},
    "macOS-x64": {".dmg"},
    "Windows-x64": {".exe"},
    "Linux-x64": {".deb", ".AppImage"},
    "Linux-arm64": {".deb"},
}[platform]
output = Path("release-assets")
output.mkdir(exist_ok=True)
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
