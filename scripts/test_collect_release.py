"""Exercise release collection using isolated build outputs, without native compilers."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from zipfile import ZipFile


SCRIPT = Path(__file__).with_name("collect-release.py").resolve()
PORTABLE_NAME = "Fangxu-File-Transfer-1.2.3-Windows-x64-portable"
MAIN = "src-tauri/target/release/fangxu-file-transfer-desktop.exe"
SIDECAR = "src-tauri/binaries/fangxu-transfer-service-x86_64-pc-windows-msvc.exe"


class ReleasePackagingTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.write("package.json", json.dumps({"version": "1.2.3"}).encode())
        self.write(MAIN, b"MZ-client")
        self.write(SIDECAR, b"MZ-service")
        self.write("LICENSE", b"test license")
        self.write("docs/windows-portable.txt",
                   (SCRIPT.parent.parent / "docs/windows-portable.txt").read_bytes())

    def write(self, name, data):
        destination = self.root / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(data)

    def collect(self, platform="Windows-x64", *options):
        return subprocess.run(
            [sys.executable, "-B", str(SCRIPT), "--platform", platform, *options],
            cwd=self.root, capture_output=True, text=True, encoding="utf-8",
        )

    def test_windows_release_keeps_installer_and_adds_complete_portable_zip(self):
        self.write("src-tauri/target/release/bundle/nsis/setup.exe", b"installer")
        result = self.collect()
        self.assertEqual(result.returncode, 0, result.stderr)
        output = self.root / "release-assets"
        self.assertEqual(
            (output / "Fangxu-File-Transfer-1.2.3-Windows-x64.exe").read_bytes(),
            b"installer",
        )
        with ZipFile(output / f"{PORTABLE_NAME}.zip") as archive:
            self.assertIsNone(archive.testzip())
            extracted = self.root / "folder with spaces" / "中文"
            archive.extractall(extracted)
        folder = extracted / PORTABLE_NAME
        self.assertEqual((folder / "方序传文件.exe").read_bytes(), b"MZ-client")
        self.assertEqual((folder / "fangxu-transfer-service.exe").read_bytes(), b"MZ-service")
        self.assertEqual((folder / "LICENSE.txt").read_bytes(), b"test license")
        self.assertIn("WebView2", (folder / "使用说明-README.txt").read_text(encoding="utf-8"))
        self.assertEqual(len(list(folder.iterdir())), 4)

    def test_portable_only_does_not_require_installer(self):
        result = self.collect("Windows-x64", "--portable-only")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            [p.name for p in (self.root / "release-assets").iterdir()],
            [f"{PORTABLE_NAME}.zip"],
        )

    def test_missing_component_never_produces_incomplete_portable_zip(self):
        for required in [MAIN, SIDECAR]:
            with self.subTest(required=required):
                source = self.root / required
                contents = source.read_bytes()
                source.unlink()
                result = self.collect("Windows-x64", "--portable-only")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Missing Windows portable file", result.stderr)
                self.assertFalse((self.root / "release-assets" / f"{PORTABLE_NAME}.zip").exists())
                source.write_bytes(contents)

    def test_other_platform_packages_are_unchanged(self):
        for platform, suffixes in [
            ("macOS-arm64", [".dmg"]), ("macOS-x64", [".dmg"]),
            ("Linux-x64", [".deb", ".AppImage"]), ("Linux-arm64", [".deb"]),
        ]:
            with self.subTest(platform=platform):
                for suffix in suffixes:
                    self.write(f"src-tauri/target/release/bundle/test/app{suffix}", b"package")
                result = self.collect(platform)
                self.assertEqual(result.returncode, 0, result.stderr)
                for suffix in suffixes:
                    self.assertEqual(
                        (self.root / "release-assets" /
                         f"Fangxu-File-Transfer-1.2.3-{platform}{suffix}").read_bytes(), b"package",
                    )
        self.assertFalse(list((self.root / "release-assets").glob("*.zip")))

    def test_windows_collection_still_requires_installer(self):
        result = self.collect()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Missing installers", result.stderr)

    def test_duplicate_installers_are_rejected(self):
        for name in ["first", "second"]:
            self.write(f"src-tauri/target/release/bundle/nsis/{name}.exe", b"installer")
        result = self.collect()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Duplicate .exe installer", result.stderr)

    def test_portable_only_rejects_non_windows_platform(self):
        result = self.collect("Linux-x64", "--portable-only")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("requires --platform Windows-x64", result.stderr)


if __name__ == "__main__":
    unittest.main()
