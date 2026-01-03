import argparse
import asyncio
import json
import os
import sqlite3
import sys
import time
from pathlib import Path
from typing import Optional, Tuple

from telethon import TelegramClient, events


def build_proxy() -> Optional[Tuple]:
    host = os.environ.get("TELEGRAM_PROXY_HOST", "").strip()
    port = os.environ.get("TELEGRAM_PROXY_PORT", "").strip()
    if not host or not port:
        return None
    try:
        import socks  # type: ignore
    except Exception:
        print("Missing 'socks' package for proxy support.", file=sys.stderr)
        return None
    user = os.environ.get("TELEGRAM_PROXY_USER", "").strip()
    password = os.environ.get("TELEGRAM_PROXY_PASS", "").strip()
    return (socks.SOCKS5, host, int(port), True, user, password)


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--username", required=True)
    parser.add_argument("--text")
    parser.add_argument("--listen", action="store_true")
    return parser.parse_args()


async def start_client_with_retry(client: TelegramClient, attempts: int = 5) -> None:
    for _ in range(attempts):
        try:
            await client.start()
            return
        except sqlite3.OperationalError as exc:
            if "database is locked" in str(exc).lower():
                await client.disconnect()
                await asyncio.sleep(1)
                continue
            raise
    raise RuntimeError("Telethon client failed to start (db locked).")


def is_process_running(pid: int) -> bool:
    if pid <= 0:
        return False
    if sys.platform.startswith("win"):
        import ctypes

        PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
        STILL_ACTIVE = 259
        handle = ctypes.windll.kernel32.OpenProcess(
            PROCESS_QUERY_LIMITED_INFORMATION, False, pid
        )
        if not handle:
            return False
        exit_code = ctypes.c_ulong()
        ctypes.windll.kernel32.GetExitCodeProcess(handle, ctypes.byref(exit_code))
        ctypes.windll.kernel32.CloseHandle(handle)
        return exit_code.value == STILL_ACTIVE
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def load_config():
    api_id_raw = os.environ.get("TELEGRAM_API_ID", "10818143").strip()
    api_hash = os.environ.get(
        "TELEGRAM_API_HASH", "0e659b068d3aba071d40445dc105c289"
    ).strip()
    session_name = os.environ.get("TELEGRAM_SESSION", "session_name").strip()

    if not api_id_raw or not api_hash or not session_name:
        print("Missing TELEGRAM_API_ID/TELEGRAM_API_HASH/TELEGRAM_SESSION.", file=sys.stderr)
        return None

    try:
        api_id = int(api_id_raw)
    except ValueError:
        print("Invalid TELEGRAM_API_ID.", file=sys.stderr)
        return None

    return api_id, api_hash, session_name


def resolve_session_name(session_name: str) -> str:
    session_path = Path(session_name)
    if not session_path.is_absolute():
        session_path = Path(__file__).resolve().parent / session_path
    if session_path.suffix != ".session":
        session_path = session_path.with_suffix(".session")
    return str(session_path)


def create_client():
    config = load_config()
    if not config:
        return None
    api_id, api_hash, session_name = config

    if sys.platform.startswith("win"):
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())

    proxy = build_proxy()
    return TelegramClient(
        resolve_session_name(session_name),
        api_id,
        api_hash,
        proxy=proxy,
        connection_retries=5,
    )


async def run(username: str, text: str) -> int:
    if send_via_listener(username, text):
        print("queued")
        return 0

    client = create_client()
    if not client:
        return 2

    target = username.lstrip("@").strip()
    if not target:
        print("Invalid username.", file=sys.stderr)
        return 2

    output_dir = Path(__file__).resolve().parent

    await start_client_with_retry(client)
    try:
        await client.send_message(target, text)
        await save_latest_incoming_message(client, target, output_dir)
        print("ok")
        return 0
    finally:
        await client.disconnect()


async def listen(username: str) -> int:
    client = create_client()
    if not client:
        return 2

    target = username.lstrip("@").strip()
    if not target:
        print("Invalid username.", file=sys.stderr)
        return 2

    output_dir = Path(__file__).resolve().parent
    output_path = output_dir / f"{target}.text"

    session_path = Path(client.session.filename)
    if not session_path.exists():
        print(
            f"Missing session file: {session_path}. "
            "Run this script once in a console to login.",
            file=sys.stderr,
        )

    try:
        await start_client_with_retry(client)
    except OSError as exc:
        print(f"Login required: {exc}", file=sys.stderr)
        await client.disconnect()
        return 2
    try:
        entity = await client.get_entity(target)
    except Exception as exc:
        print(f"Failed to resolve user: {exc}", file=sys.stderr)
        await client.disconnect()
        return 2

    @client.on(events.NewMessage(from_users=entity, incoming=True))
    async def handler(event):
        content = event.raw_text or ""
        try:
            output_path.write_text(content, encoding="utf-8")
            print(f"Saved latest message to {output_path}")
        except OSError as exc:
            print(f"Failed to write file: {exc}", file=sys.stderr)

    print(f"Listening for messages from @{target} ...")
    try:
        asyncio.create_task(process_send_queue(client))
        await client.run_until_disconnected()
    finally:
        await client.disconnect()
    return 0


def send_via_listener(username: str, text: str) -> bool:
    base_dir = Path(__file__).resolve().parent
    pid_file = base_dir.parents[4] / "server" / "logs" / "telegram_listener.pid"
    if not pid_file.exists():
        return False
    try:
        pid = int(pid_file.read_text(encoding="utf-8").strip())
    except ValueError:
        return False
    if not is_process_running(pid):
        return False

    request = {
        "username": username,
        "text": text,
        "ts": int(time.time()),
    }
    queue_file = base_dir / f"send_request_{request['ts']}_{os.getpid()}.json"
    try:
        queue_file.write_text(json.dumps(request), encoding="utf-8")
        return True
    except OSError:
        return False


async def process_send_queue(client: TelegramClient) -> None:
    base_dir = Path(__file__).resolve().parent
    while True:
        try:
            for path in sorted(base_dir.glob("send_request_*.json")):
                try:
                    data = json.loads(path.read_text(encoding="utf-8"))
                except (OSError, json.JSONDecodeError):
                    path.unlink(missing_ok=True)
                    continue
                username = str(data.get("username", "")).strip()
                text = str(data.get("text", "")).strip()
                if username and text:
                    target = username.lstrip("@").strip()
                    if target:
                        await client.send_message(target, text)
                path.unlink(missing_ok=True)
        except Exception as exc:
            print(f"Send queue error: {exc}", file=sys.stderr)
        await asyncio.sleep(0.5)


async def save_latest_incoming_message(
    client: TelegramClient, username: str, output_dir: Path
) -> None:
    try:
        entity = await client.get_entity(username)
    except Exception as exc:
        print(f"Failed to resolve user: {exc}", file=sys.stderr)
        return

    latest = None
    async for message in client.iter_messages(entity, limit=10):
        if getattr(message, "out", False):
            continue
        sender_id = getattr(message, "sender_id", None)
        if sender_id and sender_id == getattr(entity, "id", None):
            latest = message
            break

    if not latest:
        return

    filename = f"{username}.text"
    output_path = output_dir / filename
    content = getattr(latest, "message", None) or getattr(latest, "text", "") or ""
    try:
        output_path.write_text(content, encoding="utf-8")
    except OSError as exc:
        print(f"Failed to write file: {exc}", file=sys.stderr)


def main() -> int:
    args = parse_args()
    if args.listen:
        return asyncio.run(listen(args.username))
    if not args.text:
        print("Missing --text for send mode.", file=sys.stderr)
        return 2
    return asyncio.run(run(args.username, args.text))


if __name__ == "__main__":
    raise SystemExit(main())
