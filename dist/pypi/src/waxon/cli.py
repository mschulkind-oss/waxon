"""Thin wrapper that downloads and runs the waxon Go binary."""

import os
import platform
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from importlib.metadata import version as pkg_version
from pathlib import Path

REPO = "mschulkind-oss/waxon"
BIN_DIR = Path(__file__).parent / "bin"


def _get_platform():
    system = platform.system().lower()
    machine = platform.machine().lower()

    os_map = {"linux": "linux", "darwin": "darwin", "windows": "windows"}
    arch_map = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}

    goos = os_map.get(system)
    goarch = arch_map.get(machine)

    if not goos or not goarch:
        print(f"Unsupported platform: {system}-{machine}", file=sys.stderr)
        sys.exit(1)

    return goos, goarch


def _bin_path():
    goos, _ = _get_platform()
    name = "waxon.exe" if goos == "windows" else "waxon"
    return BIN_DIR / name


def _download_binary():
    try:
        ver = pkg_version("waxon")
    except Exception:
        ver = "0.0.0"

    goos, goarch = _get_platform()
    ext = "zip" if goos == "windows" else "tar.gz"
    url = f"https://github.com/{REPO}/releases/download/v{ver}/waxon-{ver}-{goos}-{goarch}.{ext}"
    bin_name = "waxon.exe" if goos == "windows" else "waxon"

    BIN_DIR.mkdir(parents=True, exist_ok=True)

    print(f"Downloading waxon v{ver} for {goos}/{goarch}...", file=sys.stderr)

    with tempfile.TemporaryDirectory() as tmp:
        archive = os.path.join(tmp, f"waxon.{ext}")
        urllib.request.urlretrieve(url, archive)

        if ext == "tar.gz":
            with tarfile.open(archive, "r:gz") as tar:
                tar.extractall(tmp)
        else:
            with zipfile.ZipFile(archive, "r") as z:
                z.extractall(tmp)

        src = os.path.join(tmp, bin_name)
        dst = BIN_DIR / bin_name
        shutil.copy2(src, dst)
        dst.chmod(dst.stat().st_mode | stat.S_IEXEC)

    print(f"Installed waxon v{ver}", file=sys.stderr)
    return dst


def main():
    binary = _bin_path()

    if not binary.exists():
        binary = _download_binary()

    try:
        result = subprocess.run([str(binary)] + sys.argv[1:])
        sys.exit(result.returncode)
    except FileNotFoundError:
        print(f"waxon binary not found at {binary}", file=sys.stderr)
        print(f"Try reinstalling: uvx waxon --reinstall", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        sys.exit(130)
