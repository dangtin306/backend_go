import json
import os
import statistics
from datetime import datetime
from typing import List, Tuple, Dict, Any
import sys


# ================== SETTINGS ==================

TARGET_CATEGORY = "https://yeumoney.com"

# Độ gắt khi lọc: "soft" (nhẹ), "medium" (vừa), "hard" (gắt)
FILTER_STRICTNESS = "hard"

STRICTNESS_CONFIG: Dict[str, float] = {
    "soft": 2.0,    # k lớn → khoảng rộng → loại ít
    "medium": 1.5,  # k chuẩn Tukey
    "hard": 1.0,    # k nhỏ → khoảng hẹp → loại nhiều
}

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
DATA_FILE = os.path.join(BASE_DIR, "data_time_process.json")

# ==============================================


def ensure_utf8_console() -> None:
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8")


ensure_utf8_console()


class CategoryAnalysisError(Exception):
    """Raised when we cannot compute a stable average for the category."""


def load_json(path: str) -> Any:
    """Load JSON file and return parsed Python object."""
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def extract_mission_durations(data: Any, api_category: str) -> List[float]:
    """
    Lấy danh sách thời gian (giây) cho 1 api_category.

    Ưu tiên:
        1) mission_time
        2) mission_second
        3) (mission_updatedate - mission_createdate).total_seconds()

    Bỏ các giá trị âm.
    """
    durations: List[float] = []

    if not isinstance(data, list):
        raise ValueError("Top-level JSON is expected to be a list.")

    for group in data:
        if not isinstance(group, dict):
            continue

        if group.get("api_category") != api_category:
            continue

        missions = group.get("data_mission", [])
        if not isinstance(missions, list):
            continue

        for mission in missions:
            if not isinstance(mission, dict):
                continue

            sec = mission.get("mission_time")
            if not isinstance(sec, (int, float)):
                sec = mission.get("mission_second")

            # Nếu vẫn không có, tính từ created/updated
            if not isinstance(sec, (int, float)):
                created = mission.get("mission_createdate")
                updated = mission.get("mission_updatedate")
                if isinstance(created, str) and isinstance(updated, str):
                    try:
                        c = datetime.strptime(created, "%Y-%m-%d %H:%M:%S")
                        u = datetime.strptime(updated, "%Y-%m-%d %H:%M:%S")
                        sec = (u - c).total_seconds()
                    except ValueError:
                        sec = None

            if isinstance(sec, (int, float)) and sec >= 0:
                durations.append(float(sec))

    return durations


def percentile(sorted_data: List[float], p: float) -> float:
    """
    Tính p-th percentile (0–100) từ list đã sort,
    dùng nội suy tuyến tính.
    """
    if not sorted_data:
        raise ValueError("Cannot compute percentile of empty data.")

    if p <= 0:
        return sorted_data[0]
    if p >= 100:
        return sorted_data[-1]

    n = len(sorted_data)
    k = (n - 1) * (p / 100.0)
    f = int(k)
    c = min(f + 1, n - 1)
    frac = k - f

    return sorted_data[f] * (1.0 - frac) + sorted_data[c] * frac


def compute_iqr_bounds(sorted_data: List[float], k: float) -> Tuple[float, float, float, float]:
    """
    Tính Q1, Q3 và cận dưới/ trên theo IQR với hệ số k:

        lower = Q1 - k * IQR
        upper = Q3 + k * IQR
    """
    q1 = percentile(sorted_data, 25.0)
    q3 = percentile(sorted_data, 75.0)
    iqr = q3 - q1
    lower = q1 - k * iqr
    upper = q3 + k * iqr
    return q1, q3, lower, upper


def filter_by_iqr(data: List[float], strictness: str) -> Tuple[List[float], Dict[str, float]]:
    """
    Lọc outlier theo quy tắc IQR với độ gắt 'strictness'.

    Trả về:
        filtered_data
        stats: Q1, Q3, IQR, lower, upper, k, n_original, n_filtered, n_removed
    """
    if not data:
        return [], {}

    if strictness not in STRICTNESS_CONFIG:
        raise ValueError(
            f"FILTER_STRICTNESS '{strictness}' không hợp lệ. "
            f"Chọn một trong: {list(STRICTNESS_CONFIG.keys())}"
        )

    k = STRICTNESS_CONFIG[strictness]
    sorted_data = sorted(data)

    q1, q3, lower, upper = compute_iqr_bounds(sorted_data, k)
    iqr = q3 - q1

    filtered = [x for x in sorted_data if lower <= x <= upper]

    stats: Dict[str, float] = {
        "k": k,
        "Q1": q1,
        "Q3": q3,
        "IQR": iqr,
        "lower": lower,
        "upper": upper,
        "n_original": len(data),
        "n_filtered": len(filtered),
        "n_removed": len(data) - len(filtered),
    }
    return filtered, stats


def analyze_category(data: Any, api_category: str, strictness: str = FILTER_STRICTNESS) -> Dict[str, Any]:
    """Return average time + IQR stats for a specific api_category."""
    durations = extract_mission_durations(data, api_category)
    if not durations:
        raise CategoryAnalysisError(
            f"Không tìm thấy dữ liệu thời gian cho api_category = {api_category}"
        )

    filtered, iqr_stats = filter_by_iqr(durations, strictness)

    if not filtered:
        raise CategoryAnalysisError("Sau khi lọc IQR không còn dữ liệu (lọc quá gắt).")

    avg_time = statistics.mean(filtered)

    return {
        "api_category": api_category,
        "average_time": avg_time,
        "iqr_stats": iqr_stats,
    }


def print_report(target_category: str, analysis: Dict[str, Any]) -> None:
    """Pretty-print the analysis result to stdout."""
    iqr_stats = analysis["iqr_stats"]

    print("----- THÔNG TIN IQR (LỌC OUTLIER) -----")
    print(f"api_category: {target_category}")
    print(f"Độ gắt lọc (strictness): {FILTER_STRICTNESS}")
    print(f"k   = {iqr_stats['k']:.2f}")
    print(f"Q1  = {iqr_stats['Q1']:.2f} s")
    print(f"Q3  = {iqr_stats['Q3']:.2f} s")
    print(f"IQR = {iqr_stats['IQR']:.2f} s")
    print(f"Lower bound = {iqr_stats['lower']:.2f} s")
    print(f"Upper bound = {iqr_stats['upper']:.2f} s")
    print(f"Số phần tử ban đầu: {int(iqr_stats['n_original'])}")
    print(f"Số phần tử sau khi lọc: {int(iqr_stats['n_filtered'])}")
    print(f"Số phần tử bị loại: {int(iqr_stats['n_removed'])}")
    print()

    print("===== KẾT QUẢ CUỐI CÙNG (TRUNG BÌNH SAU LỌC GẮT) =====")
    print(f"Thời gian trung bình hợp lý nhất: {analysis['average_time']:.0f} giây")


def run_cli(target_category: str) -> None:
    data = load_json(DATA_FILE)
    try:
        analysis = analyze_category(data, target_category)
    except CategoryAnalysisError as exc:
        print(str(exc))
        return

    print_report(target_category, analysis)


def main() -> None:
    run_cli(TARGET_CATEGORY)


if __name__ == "__main__":
    main()