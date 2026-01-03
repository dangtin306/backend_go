import os
import subprocess
import sys
import time
from pathlib import Path

from golang import start_golang, stop_golang
from nginx import start_nginx, stop_nginx


BASE_DIR = Path(__file__).resolve().parents[1]
LOG_DIR = Path(__file__).resolve().parent / "logs"
TELEGRAM_SCRIPT = (
    BASE_DIR / "golang" / "p2p" / "media" / "social" / "telegram" / "auto_reply.py"
)
TELEGRAM_PID = LOG_DIR / "telegram_listener.pid"


def is_process_running(pid):
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


def _kill_pid(pid):
    if sys.platform.startswith("win"):
        subprocess.run(
            ["taskkill", "/PID", str(pid), "/T", "/F"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
    else:
        try:
            os.kill(pid, 15)
        except OSError:
            pass


def stop_existing_main_instances():
    if not sys.platform.startswith("win"):
        return
    current_pid = os.getpid()
    script_path = str(Path(__file__).resolve())
    escaped = script_path.replace("'", "''")
    cmd = (
        "Get-CimInstance Win32_Process | "
        "Where-Object { $_.CommandLine -like '*"
        + escaped
        + "*' -and $_.ProcessId -ne "
        + str(current_pid)
        + " } | Select-Object -ExpandProperty ProcessId"
    )
    try:
        output = subprocess.check_output(
            ["powershell", "-NoProfile", "-Command", cmd],
            text=True,
            encoding="utf-8",
        )
    except subprocess.CalledProcessError:
        return
    pids = [line.strip() for line in output.splitlines() if line.strip().isdigit()]
    for pid in pids:
        _kill_pid(pid)
    if pids:
        print(f"Stopped other main.py instances: {', '.join(pids)}")


def stop_telegram_listener():
    if not TELEGRAM_PID.exists():
        return 0
    try:
        existing_pid = int(TELEGRAM_PID.read_text(encoding="utf-8").strip())
    except ValueError:
        existing_pid = 0
    if existing_pid > 0 and is_process_running(existing_pid):
        _kill_pid(existing_pid)
        print(f"Stopped telegram listener (pid {existing_pid}).")
    TELEGRAM_PID.unlink(missing_ok=True)
    return 0


def start_telegram_listener():
    if not TELEGRAM_SCRIPT.exists():
        print(f"Missing telegram script: {TELEGRAM_SCRIPT}")
        return 1
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    if TELEGRAM_PID.exists():
        try:
            existing_pid = int(TELEGRAM_PID.read_text(encoding="utf-8").strip())
        except ValueError:
            existing_pid = 0
        if existing_pid > 0 and is_process_running(existing_pid):
            print(f"Telegram listener already running (pid {existing_pid}).")
            return 0
        TELEGRAM_PID.unlink(missing_ok=True)

    username = os.environ.get("TELEGRAM_LISTEN_USER", "@tinnguyen_ok").strip()
    if username and not username.startswith("@"):
        username = f"@{username}"
    if not username:
        print("Missing TELEGRAM_LISTEN_USER.")
        return 1

    py_exe = os.environ.get("PYTHON_EXE", "python").strip() or "python"
    cmd = [py_exe, str(TELEGRAM_SCRIPT), "--listen", "--username", username]
    log_path = LOG_DIR / "telegram-listener.log"
    with open(log_path, "a", encoding="utf-8") as log_file:
        if sys.platform.startswith("win"):
            creationflags = (
                subprocess.DETACHED_PROCESS | subprocess.CREATE_NEW_PROCESS_GROUP
            )
            proc = subprocess.Popen(
                cmd,
                stdout=log_file,
                stderr=log_file,
                creationflags=creationflags,
            )
        else:
            proc = subprocess.Popen(
                cmd,
                stdout=log_file,
                stderr=log_file,
                start_new_session=True,
            )

    TELEGRAM_PID.write_text(str(proc.pid), encoding="utf-8")
    print(f"Started telegram listener (pid {proc.pid}).")
    return 0


def main():
    stop_existing_main_instances()
    stop_golang()
    stop_nginx()
    stop_telegram_listener()
    nginx_result = start_nginx()
    if nginx_result != 0:
        return nginx_result
    time.sleep(2)
    start_telegram_listener()
    return start_golang(show_logs=True, wait=True, watch=True)


if __name__ == "__main__":
    raise SystemExit(main())
