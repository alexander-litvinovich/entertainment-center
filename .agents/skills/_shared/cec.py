#!/usr/bin/env python3
"""Build, flash and talk to the host-driven XIAO RP2350 CEC adapter."""
from pathlib import Path
import argparse
import datetime
import hashlib
import json
import os
import platform
import re
import shutil
import struct
import subprocess
import sys
import tarfile
import time
import urllib.request

TOOLING = Path(__file__).resolve().parent
ROOT = TOOLING.parents[2] / "hdmi-cec"
FW = ROOT / "firmware"
BUILD = ROOT / "build" / "host-rp2350"
TOOLS = ROOT / ".tools"
VENV = ROOT / ".venv"
TOOLCHAIN_NAME = "arm-gnu-toolchain-14.3.rel1-darwin-arm64-arm-none-eabi"
TOOLCHAIN = TOOLS / TOOLCHAIN_NAME
ARM_URL = "https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/" + TOOLCHAIN_NAME + ".tar.xz"
ARM_SHA = "30f4d08b219190a37cded6aa796f4549504902c53cfc3c7e044a8490b6eba1f7"
VERSION_PREFIX = "cec-host-"
VERSION = VERSION_PREFIX + (ROOT / "VERSION").read_text().strip()
REPO = "alexander-litvinovich/entertainment-center"
TAG_PREFIX = "hdmi-cec-v"

def run(*cmd, capture=False, cwd=ROOT, timeout=600):
    cmd = [str(x) for x in cmd]
    result = subprocess.run(cmd, cwd=cwd, check=True, text=True,
                            stdout=subprocess.PIPE if capture else None,
                            stderr=subprocess.STDOUT if capture else None, timeout=timeout)
    return result.stdout.strip() if capture else ""

def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()

def tool_env():
    return TOOLCHAIN / "bin" / "arm-none-eabi-gcc"

def check_sources():
    modules = run("git", "-C", ROOT, "submodule", "status", cwd=ROOT, capture=True)
    if any(line.startswith(("-", "+", "U")) for line in modules.splitlines()):
        raise RuntimeError("Missing or mismatched submodule. Run setup.")
    if not (FW / "pico-sdk/lib/tinyusb/src/tusb.h").exists():
        raise RuntimeError("TinyUSB is missing. Run setup.")
    return run("git", "-C", ROOT, "submodule", "status", "--recursive", capture=True)

def setup(_):
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise RuntimeError("This bootstrap supports Apple Silicon macOS.")
    if not shutil.which("brew"):
        raise RuntimeError("Homebrew is required: https://brew.sh")
    missing = [x for x in ("cmake", "ninja", "picotool") if not shutil.which(x)]
    if missing:
        env = dict(os.environ, HOMEBREW_NO_AUTO_UPDATE="1", HOMEBREW_NO_INSTALL_CLEANUP="1")
        subprocess.run(["brew", "install", *missing], check=True, env=env)
    TOOLS.mkdir(exist_ok=True)
    if not tool_env().exists():
        archive = TOOLS / (TOOLCHAIN_NAME + ".tar.xz")
        if not archive.exists() or sha(archive) != ARM_SHA:
            temporary = archive.with_suffix(".download")
            print("Downloading Arm GNU Toolchain 14.3.rel1...", flush=True)
            urllib.request.urlretrieve(ARM_URL, temporary)
            if sha(temporary) != ARM_SHA:
                raise RuntimeError("Toolchain checksum mismatch")
            temporary.replace(archive)
        with tarfile.open(archive) as t:
            t.extractall(TOOLS, filter="data")
    if not (VENV / "bin/python").exists():
        run(sys.executable, "-m", "venv", VENV)
    run(VENV / "bin/python", "-m", "pip", "install", "-r", TOOLING / "requirements.txt")
    # Only what the RP2350 build needs: the SDK's TinyUSB, not its Wi-Fi/TLS stacks.
    run("git", "-C", ROOT, "submodule", "update", "--init", "firmware/pico-sdk",
        "firmware/FreeRTOS-Kernel", "firmware/tcli", timeout=1800)
    run("git", "-C", FW / "pico-sdk", "submodule", "update", "--init", "lib/tinyusb", timeout=1800)
    run(VENV / "bin/python", Path(__file__).resolve(), "doctor")

def doctor(_):
    check_sources()
    for cmd in (["cmake", "--version"], ["ninja", "--version"], ["picotool", "version"],
                [tool_env(), "--version"]):
        print(run(*cmd, capture=True).splitlines()[0])
    libc = run(tool_env(), "-print-file-name=libc.a", capture=True)
    if not Path(libc).is_file():
        raise RuntimeError("Arm newlib is missing")
    import serial
    import usb.core
    print("Python, pyserial and pyusb OK; newlib:", libc)

def source_hash(source=FW):
    digest = hashlib.sha256()
    files = [ROOT / "config.cmake"]
    for base in ("src", "include", "crc"):
        files.extend(p for p in (source / base).rglob("*") if p.is_file())
    files.append(source / "CMakeLists.txt")
    for p in sorted(files):
        digest.update(str(p.relative_to(source) if p.is_relative_to(source) else p.name).encode())
        digest.update(p.read_bytes())
    return digest.hexdigest()

def build(args):
    doctor(None)
    version = VERSION + "-" + source_hash()[:8]
    run("cmake", "-S", FW, "-B", BUILD, "-G", "Ninja", "-C", ROOT / "config.cmake",
        "-DPICO_BOARD=seeed_xiao_rp2350", "-DPICO_TOOLCHAIN_PATH=" + str(TOOLCHAIN),
        "-DCMAKE_POLICY_VERSION_MINIMUM=3.5", "-DPICO_CEC_VERSION=" + version)
    run("cmake", "--build", BUILD, "--parallel", "8")
    info = run("picotool", "info", "-a", BUILD / "pico-cec.uf2", capture=True)
    manifest = {"version": version, "board": "seeed_xiao_rp2350",
                "built_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "source_sha256": source_hash(),
                "config": (ROOT / "config.cmake").read_text(),
                "submodules": check_sources(), "picotool_info": info,
                "tools": {cmd: run(cmd, flag, capture=True).splitlines()[0] for cmd, flag in
                          [("cmake", "--version"), ("ninja", "--version"), ("picotool", "version"),
                           (str(tool_env()), "--version")]},
                "sha256": {f: sha(BUILD / f) for f in ("pico-cec.uf2", "pico-cec.elf")}}
    (BUILD / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(info)
    print("Output:", BUILD / "pico-cec.uf2")

def validate_uf2(path):
    data = Path(path).read_bytes()
    if not data or len(data) % 512:
        raise ValueError("Not a complete UF2")
    families = set()
    for offset in range(0, len(data), 512):
        a, b, flags, target, size, number, total, family = struct.unpack_from("<8I", data, offset)
        end, = struct.unpack_from("<I", data, offset + 508)
        if (a, b, end) != (0x0A324655, 0x9E5D5157, 0x0AB16F30) or not flags & 0x2000:
            raise ValueError("Invalid UF2 header/family")
        if size > 476 or size == 0:
            raise ValueError("Invalid UF2 block size")
        families.add(family)
    if 0xe48bff59 not in families or not families <= {0xe48bff59, 0xe48bff57}:
        raise ValueError("Expected RP2350 ARM Secure UF2, not RP2040 or RISC-V")

def devices():
    import usb.core
    import usb.backend.libusb1
    backend = usb.backend.libusb1.get_backend()
    if backend is None:
        prefix = run("brew", "--prefix", "libusb", capture=True)
        backend = usb.backend.libusb1.get_backend(find_library=lambda _: prefix + "/lib/libusb-1.0.dylib")
    if backend is None:
        raise RuntimeError("libusb is unavailable")
    return list(usb.core.find(find_all=True, idVendor=0x2e8a, backend=backend) or [])

def choose_device(items, bus=None, address=None):
    matches = [d for d in items if d.idProduct in (0x0003, 0x000f)
               and (bus is None or d.bus == bus) and (address is None or d.address == address)]
    if not matches:
        raise RuntimeError("No BOOTSEL device. Hold BOOT while connecting XIAO over USB.")
    if len(matches) != 1:
        choices = ", ".join(f"--bus {d.bus} --address {d.address}" for d in matches)
        raise RuntimeError("Several boards found; select one: " + choices)
    if matches[0].idProduct != 0x000f:
        raise RuntimeError("Selected board is RP2040; expected RP2350.")
    return matches[0]

def serial_port(explicit=None):
    from serial.tools import list_ports
    ports = [p for p in list_ports.comports() if p.vid == 0xcafe and p.pid == 0x4005]
    if explicit:
        ports = [p for p in ports if p.device == explicit]
    if len(ports) != 1:
        raise RuntimeError("Select the adapter with --port. Candidates: " +
                           ", ".join(p.device for p in ports))
    return ports[0].device

def serial_command(port, command, seconds=1):
    import serial
    with serial.Serial(port, 115200, timeout=0.1, write_timeout=2) as connection:
        time.sleep(0.3)
        connection.write(("\r" + command + "\r").encode())
        end = time.monotonic() + seconds
        output = bytearray()
        while time.monotonic() < end:
            output.extend(connection.read(4096))
    return output.decode(errors="replace")

def flash(args):
    image = Path(args.image).resolve() if args.image else BUILD / "pico-cec.uf2"
    validate_uf2(image)
    manifest = json.loads(image.with_name("manifest.json").read_text())
    if manifest["board"] != "seeed_xiao_rp2350" or not manifest["version"].startswith(VERSION_PREFIX):
        raise RuntimeError("This command only flashes our host-driven firmware for XIAO RP2350")
    if sha(image) != manifest["sha256"]["pico-cec.uf2"]:
        raise RuntimeError("Image checksum differs from build manifest")
    info = run("picotool", "info", "-a", image, capture=True)
    if "seeed_xiao_rp2350" not in info:
        raise RuntimeError("Image board metadata does not match XIAO RP2350")
    if args.bootsel:
        port = serial_port(args.port)
        output = serial_command(port, "show version")
        if not re.search(r"cec-(?:wake|host)-", output):
            raise RuntimeError("Serial device did not identify as our firmware")
        try:
            serial_command(port, "reboot bootsel", 0.5)
        except OSError:
            pass  # expected disconnection during reboot
        time.sleep(5)
    device = choose_device(devices(), args.bus, args.address)
    selection = ["--bus", str(device.bus), "--address", str(device.address)]
    target = run("picotool", "info", "-a", *selection, capture=True, timeout=20)
    if not re.search(r"(?:type|chip):\s+RP2350", target, re.I):
        raise RuntimeError("picotool did not confirm RP2350:\n" + target)
    print(target)
    record = {"utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "version": manifest["version"], "sha256": sha(image),
              "bus": device.bus, "address": device.address, "device_info": target,
              "verified": False, "rebooted": False}
    logs = ROOT / "build/flash-records"
    logs.mkdir(parents=True, exist_ok=True)
    record_path = logs / (datetime.datetime.now().strftime("%Y%m%d-%H%M%S") + ".json")
    try:
        run("picotool", "load", "-v", image, *selection, timeout=60)
        run("picotool", "verify", image, *selection, timeout=60)
        record["verified"] = True
        run("picotool", "reboot", *selection, timeout=20)
        record["rebooted"] = True
    finally:
        record_path.write_text(json.dumps(record, indent=2) + "\n")
    print("Flash verified and reboot requested. Record:", record_path)
    print(f"Run `monitor --command \"show version\"`; expect {manifest['version']}. "
          "Reset must not create CEC traffic.")

def github(url):
    request = urllib.request.Request(url, headers={"User-Agent": "hdmi-cec-tooling",
                                                   "Accept": "application/vnd.github+json"})
    with urllib.request.urlopen(request, timeout=60) as response:
        return response.read()

def download(args):
    if args.tag:
        release = json.loads(github(f"https://api.github.com/repos/{REPO}/releases/tags/{args.tag}"))
    else:
        releases = json.loads(github(f"https://api.github.com/repos/{REPO}/releases?per_page=30"))
        releases = [r for r in releases if r["tag_name"].startswith(TAG_PREFIX)
                    and not r["draft"] and not r["prerelease"]]
        if not releases:
            raise RuntimeError("No published hdmi-cec release")
        release = max(releases, key=lambda r: tuple(int(x) for x in r["tag_name"][len(TAG_PREFIX):].split(".")))
    assets = {a["name"]: a["browser_download_url"] for a in release["assets"]}
    target = ROOT / "build" / "releases" / release["tag_name"]
    target.mkdir(parents=True, exist_ok=True)
    sums = dict(reversed(line.split()) for line in github(assets["SHA256SUMS"]).decode().splitlines() if line)
    for name in ("pico-cec.uf2", "manifest.json"):
        data = github(assets[name])
        if hashlib.sha256(data).hexdigest() != sums[name]:
            raise RuntimeError(f"{name} checksum differs from SHA256SUMS")
        (target / name).write_bytes(data)
    validate_uf2(target / "pico-cec.uf2")
    print("Downloaded", release["tag_name"], "to", target)
    print("Flash with: flash --image", target / "pico-cec.uf2")

def monitor(args):
    port = serial_port(args.port)
    if args.command:
        print(serial_command(port, args.command, args.seconds))
    else:
        print("Console:", port, "  Exit: Ctrl+]")
        run(sys.executable, "-m", "serial.tools.miniterm", port, "115200", "--raw", timeout=None)

def main():
    in_venv = Path(sys.prefix).resolve() == VENV.resolve()
    if not in_venv and (VENV / "bin/python").exists() and sys.argv[1:2] != ["setup"]:
        os.execv(VENV / "bin/python", [str(VENV / "bin/python"), __file__, *sys.argv[1:]])
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("setup", "doctor", "build", "download", "flash", "monitor"):
        p = sub.add_parser(name)
        p.set_defaults(handler=globals()[name])
        if name in ("flash", "monitor"):
            p.add_argument("--port")
        if name == "download":
            p.add_argument("--tag", help="Release tag, e.g. hdmi-cec-v0.7.0; default: latest")
        if name == "flash":
            p.add_argument("--image")
            p.add_argument("--bus", type=int)
            p.add_argument("--address", type=int)
            p.add_argument("--bootsel", action="store_true", help="Reboot our running firmware into BOOTSEL")
        if name == "monitor":
            p.add_argument("--command")
            p.add_argument("--seconds", type=float, default=2)
    args = parser.parse_args()
    try:
        args.handler(args)
    except (RuntimeError, ValueError, KeyError, OSError, subprocess.SubprocessError) as e:
        print("Error:", e, file=sys.stderr)
        if isinstance(e, subprocess.CalledProcessError) and e.stdout:
            print(e.stdout, file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
