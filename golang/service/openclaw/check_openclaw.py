from __future__ import annotations

import argparse
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

DEFAULT_HOST = "127.0.0.1"
DEFAULT_PORT = 8796
DEFAULT_TIMEOUT = 5.0
DEFAULT_INTERVAL = 15.0
DEFAULT_RESET_COOLDOWN = 30.0
DEFAULT_START_GRACE = 12.0
OPENCLAW_ROOT = Path(r"D:\hustmedia\application\openclaw")
OPENCLAW_CMD = OPENCLAW_ROOT / "openclaw.cmd"


def tcp_probe(host: str, port: int, timeout: float) -> tuple[bool, str]:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True, f"tcp ok {host}:{port}"
    except OSError as exc:
        return False, f"tcp fail {host}:{port}: {exc}"


def http_probe(url: str, timeout: float) -> tuple[bool, str]:
    request = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            status = getattr(response, "status", 200)
            if 200 <= status < 500:
                return True, f"http ok {status} {url}"
            return False, f"http bad status {status} {url}"
    except urllib.error.HTTPError as exc:
        if 200 <= exc.code < 500:
            return True, f"http ok {exc.code} {url}"
        return False, f"http fail {exc.code} {url}"
    except Exception as exc:  # noqa: BLE001
        return False, f"http fail {url}: {exc}"


def status_probe(openclaw_cmd: Path, timeout: float) -> tuple[bool, str]:
    if not openclaw_cmd.exists():
        return False, f"missing launcher: {openclaw_cmd}"
    try:
        proc = subprocess.run(
            [str(openclaw_cmd), "gateway", "status"],
            capture_output=True,
            text=True,
            timeout=max(10, int(timeout * 4)),
            check=False,
        )
    except Exception as exc:  # noqa: BLE001
        return False, f"status fail: {exc}"

    output = "\n".join(part for part in [proc.stdout, proc.stderr] if part).strip()
    normalized = output.lower()
    healthy = proc.returncode == 0 and (
        "runtime: running" in normalized
        or "rpc probe: ok" in normalized
        or "listening:" in normalized
    )
    if healthy:
        return True, "status ok"
    short = output.splitlines()[-1] if output else f"exit={proc.returncode}"
    return False, f"status fail: {short}"


def check_openclaw(host: str, port: int, timeout: float, openclaw_cmd: Path) -> tuple[bool, list[str]]:
    messages: list[str] = []
    tcp_ok, tcp_msg = tcp_probe(host, port, timeout)
    messages.append(tcp_msg)

    status_ok, status_msg = status_probe(openclaw_cmd, timeout)
    messages.append(status_msg)

    if tcp_ok:
        url = f"http://{host}:{port}/"
        http_ok, http_msg = http_probe(url, timeout)
        messages.append(http_msg)
    else:
        http_ok = False

    healthy = tcp_ok and (status_ok or http_ok)
    return healthy, messages


def open_reset_powershell(openclaw_cmd: Path, reset_command: str) -> None:
    if not openclaw_cmd.exists():
        raise FileNotFoundError(f"OpenClaw launcher not found: {openclaw_cmd}")

    if reset_command.strip().lower() == "default":
        command = f'& "{openclaw_cmd}" gateway restart'
    else:
        command = reset_command

    subprocess.Popen(
        [
            "powershell.exe",
            "-NoProfile",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            command,
        ],
        creationflags=getattr(subprocess, "CREATE_NEW_CONSOLE", 0),
        cwd=str(openclaw_cmd.parent),
    )


def run_once(args: argparse.Namespace) -> int:
    healthy, messages = check_openclaw(args.host, args.port, args.timeout, args.openclaw_cmd)
    for message in messages:
        print(message)
    if healthy:
        print("openclaw healthy")
        return 0

    print("openclaw unhealthy -> opening new PowerShell to reset")
    open_reset_powershell(args.openclaw_cmd, args.reset_command)
    return 1


def run_watch(args: argparse.Namespace) -> int:
    last_reset_at = 0.0
    while True:
        healthy, messages = check_openclaw(args.host, args.port, args.timeout, args.openclaw_cmd)
        stamp = time.strftime("%Y-%m-%d %H:%M:%S")
        state = "healthy" if healthy else "unhealthy"
        print(f"[{stamp}] {state}")
        for message in messages:
            print(f"  - {message}")

        now = time.time()
        if not healthy and now - last_reset_at >= args.reset_cooldown:
            print("  - opening new PowerShell to reset OpenClaw")
            open_reset_powershell(args.openclaw_cmd, args.reset_command)
            last_reset_at = now
            time.sleep(args.start_grace)
        else:
            time.sleep(args.interval)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Check local OpenClaw gateway health and restart it in a new PowerShell window when needed."
    )
    parser.add_argument("--host", default=DEFAULT_HOST)
    parser.add_argument("--port", type=int, default=DEFAULT_PORT)
    parser.add_argument("--timeout", type=float, default=DEFAULT_TIMEOUT)
    parser.add_argument("--interval", type=float, default=DEFAULT_INTERVAL)
    parser.add_argument("--reset-cooldown", type=float, default=DEFAULT_RESET_COOLDOWN)
    parser.add_argument("--start-grace", type=float, default=DEFAULT_START_GRACE)
    parser.add_argument(
        "--reset-command",
        default="default",
        help="PowerShell command to run in the new window. Use 'default' for '<openclaw.cmd> gateway restart'.",
    )
    parser.add_argument(
        "--openclaw-cmd",
        type=Path,
        default=OPENCLAW_CMD,
        help="Path to openclaw.cmd wrapper.",
    )
    parser.add_argument(
        "--watch",
        action="store_true",
        help="Keep watching and auto-reset whenever the gateway becomes unhealthy.",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    try:
        if args.watch:
            return run_watch(args)
        return run_once(args)
    except KeyboardInterrupt:
        print("stopped")
        return 130
    except Exception as exc:  # noqa: BLE001
        print(f"fatal: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
