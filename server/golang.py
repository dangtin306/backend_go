import os
import sys
import subprocess
import time
from pathlib import Path


BASE_DIR = Path(__file__).resolve().parents[1] / "golang"
LOG_DIR = Path(__file__).resolve().parent / "logs"
PID_FILE = LOG_DIR / "golang.pid"
APP_PORT = int(os.environ.get("GOLANG_PORT", "8795"))

# Optional overrides:
#   set GO_EXE to a full path, e.g. D:\hustmedia\application\Go\bin\go.exe
GO_EXE = os.environ.get("GO_EXE", "").strip()
DEFAULT_GO = Path(r"D:\hustmedia\application\Go\bin\go.exe")


def resolve_go_exe():
    if GO_EXE:
        return GO_EXE
    if DEFAULT_GO.exists():
        return str(DEFAULT_GO)
    return "go"


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


def start_golang(show_logs=False, wait=False, watch=False):
    go_mod = BASE_DIR / "go.mod"
    if not go_mod.exists():
        print(f"Missing go.mod: {go_mod}")
        return 1

    LOG_DIR.mkdir(parents=True, exist_ok=True)
    if PID_FILE.exists():
        try:
            existing_pid = int(PID_FILE.read_text(encoding="utf-8").strip())
        except ValueError:
            existing_pid = 0
        if is_process_running(existing_pid):
            if _pid_listening_on_port(existing_pid, APP_PORT):
                print(f"Golang already running (pid {existing_pid}).")
                return 0
            PID_FILE.unlink(missing_ok=True)

    go_exe = resolve_go_exe()
    cmd = [go_exe, "run", "./main"]
    if show_logs:
        if watch:
            return _start_with_watch(cmd)
        proc = subprocess.Popen(cmd, cwd=str(BASE_DIR))
        PID_FILE.write_text(str(proc.pid), encoding="utf-8")
        print(f"Started golang (pid {proc.pid}).")
        if wait:
            return proc.wait()
        return 0

    log_path = LOG_DIR / "golang-launch.log"
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
    print(f"Started golang (pid {proc.pid}).")
    return 0


def stop_golang():
    killed = set()
    killed.update(_stop_by_port(APP_PORT))

    existing_pid = 0
    if PID_FILE.exists():
        try:
            existing_pid = int(PID_FILE.read_text(encoding="utf-8").strip())
        except ValueError:
            existing_pid = 0
    if existing_pid > 0 and is_process_running(existing_pid):
        _kill_pid(existing_pid)
        killed.add(existing_pid)

    if PID_FILE.exists():
        PID_FILE.unlink(missing_ok=True)

    if killed:
        pid_list = ", ".join(sorted(str(pid) for pid in killed))
        print(f"Stopped golang processes: {pid_list}")
    else:
        print("No existing golang process found.")
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


def _pid_listening_on_port(pid, port):
    if not sys.platform.startswith("win"):
        return True
    try:
        output = subprocess.check_output(
            [
                "powershell",
                "-NoProfile",
                "-Command",
                (
                    "Get-NetTCPConnection -LocalPort "
                    + str(port)
                    + " -State Listen | Where-Object { $_.OwningProcess -eq "
                    + str(pid)
                    + " } | Select-Object -First 1"
                ),
            ],
            text=True,
            encoding="utf-8",
        )
        return bool(output.strip())
    except subprocess.CalledProcessError:
        return False


def _scan_go_files(root_dir):
    mtimes = {}
    for dirpath, dirnames, filenames in os.walk(root_dir):
        dirnames[:] = [
            d
            for d in dirnames
            if d
            not in {
                ".git",
                "__pycache__",
                "node_modules",
                "vendor",
                "bin",
                "tmp",
                "logs",
            }
        ]
        for filename in filenames:
            if not filename.endswith(".go"):
                continue
            path = os.path.join(dirpath, filename)
            try:
                mtimes[path] = os.path.getmtime(path)
            except OSError:
                continue
    return mtimes


def _start_with_watch(cmd):
    poll_interval = float(os.environ.get("GOLANG_WATCH_INTERVAL", "1.0"))
    snapshot = _scan_go_files(str(BASE_DIR))

    def spawn():
        proc = subprocess.Popen(cmd, cwd=str(BASE_DIR))
        PID_FILE.write_text(str(proc.pid), encoding="utf-8")
        print(f"Started golang (pid {proc.pid}).")
        return proc

    proc = spawn()
    try:
        while True:
            time.sleep(poll_interval)
            current = _scan_go_files(str(BASE_DIR))
            if current != snapshot:
                print("Detected Go change, restarting...")
                _kill_pid(proc.pid)
                try:
                    proc.wait(timeout=5)
                except Exception:
                    pass
                snapshot = current
                proc = spawn()
                continue
            if proc.poll() is not None:
                print("Golang process exited, restarting...")
                proc = spawn()
    except KeyboardInterrupt:
        _kill_pid(proc.pid)
        return 0


if __name__ == "__main__":
    raise SystemExit(start_golang())
