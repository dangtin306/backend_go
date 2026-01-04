import os
import sys
import subprocess
from pathlib import Path


BASE_DIR = Path(__file__).resolve().parents[1] / "nginx"
NGINX_EXE = BASE_DIR / "nginx.exe"
CONF_FILE = BASE_DIR / "conf" / "nginx.conf"
LOG_DIR = Path(__file__).resolve().parent / "logs"
PID_FILE = LOG_DIR / "nginx.pid"
NGINX_PID_FILE = BASE_DIR / "logs" / "nginx.pid"
NGINX_PORT = int(os.environ.get("NGINX_PORT", "8794"))


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


def start_nginx():
    if not NGINX_EXE.exists():
        print(f"Missing nginx binary: {NGINX_EXE}")
        return 1
    if not CONF_FILE.exists():
        print(f"Missing nginx config: {CONF_FILE}")
        return 1

    LOG_DIR.mkdir(parents=True, exist_ok=True)
    if PID_FILE.exists():
        try:
            existing_pid = int(PID_FILE.read_text(encoding="utf-8").strip())
        except ValueError:
            existing_pid = 0
        if is_process_running(existing_pid):
            print(f"Nginx already running (pid {existing_pid}).")
            return 0

    cmd = [str(NGINX_EXE), "-p", str(BASE_DIR), "-c", str(CONF_FILE)]
    log_path = LOG_DIR / "nginx-launch.log"
    with open(log_path, "a", encoding="utf-8") as log_file:
        if sys.platform.startswith("win"):
            creationflags = (
                subprocess.DETACHED_PROCESS | subprocess.CREATE_NEW_PROCESS_GROUP
            )
            proc = subprocess.Popen(
                cmd,
                cwd=str(BASE_DIR),
                stdout=log_file,
                stderr=log_file,
                creationflags=creationflags,
            )
        else:
            proc = subprocess.Popen(
                cmd,
                cwd=str(BASE_DIR),
                stdout=log_file,
                stderr=log_file,
                start_new_session=True,
            )

    PID_FILE.write_text(str(proc.pid), encoding="utf-8")
    print(f"Started nginx (pid {proc.pid}).")
    return 0


def stop_nginx():
    killed = set()

    if NGINX_EXE.exists():
        subprocess.run(
            [str(NGINX_EXE), "-p", str(BASE_DIR), "-c", str(CONF_FILE), "-s", "quit"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )

    if NGINX_PID_FILE.exists():
        try:
            pid = int(NGINX_PID_FILE.read_text(encoding="utf-8").strip())
        except ValueError:
            pid = 0
        if pid > 0 and is_process_running(pid):
            _kill_pid(pid)
            killed.add(pid)

    if PID_FILE.exists():
        try:
            existing_pid = int(PID_FILE.read_text(encoding="utf-8").strip())
        except ValueError:
            existing_pid = 0
        if existing_pid > 0 and is_process_running(existing_pid):
            _kill_pid(existing_pid)
            killed.add(existing_pid)
        PID_FILE.unlink(missing_ok=True)

    killed.update(_stop_by_port(NGINX_PORT))

    if killed:
        pid_list = ", ".join(sorted(str(pid) for pid in killed))
        print(f"Stopped nginx processes: {pid_list}")
    else:
        print("No existing nginx process found.")
    return 0


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


def _stop_by_port(port):
    if not sys.platform.startswith("win"):
        return set()

    pids = set()
    try:
        output = subprocess.check_output(
            [
                "powershell",
                "-NoProfile",
                "-Command",
                (
                    "Get-NetTCPConnection -LocalPort "
                    + str(port)
                    + " -State Listen | Select-Object -ExpandProperty OwningProcess"
                ),
            ],
            text=True,
            encoding="utf-8",
        )
        for line in output.splitlines():
            line = line.strip()
            if line.isdigit():
                pids.add(int(line))
    except subprocess.CalledProcessError:
        pids = set()

    if not pids:
        try:
            output = subprocess.check_output(
                ["netstat", "-ano", "-p", "TCP"], text=True, encoding="utf-8"
            )
        except subprocess.CalledProcessError:
            return set()
        target = f":{port}"
        for line in output.splitlines():
            if "LISTENING" not in line:
                continue
            if target not in line:
                continue
            parts = line.split()
            if len(parts) < 5:
                continue
            pid = parts[-1]
            if pid.isdigit():
                pids.add(int(pid))

    for pid in pids:
        _kill_pid(pid)

    return pids


if __name__ == "__main__":
    raise SystemExit(start_nginx())
