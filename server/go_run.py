import time

from go_main import start_golang, stop_golang


def main():
    stop_golang()
    time.sleep(1)
    return start_golang(show_logs=True, wait=True, watch=True)


if __name__ == "__main__":
    raise SystemExit(main())
