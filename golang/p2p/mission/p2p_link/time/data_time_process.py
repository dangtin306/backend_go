import json
import os
from typing import Any, List

from data_time_math import (
    DATA_FILE,
    FILTER_STRICTNESS,
    CategoryAnalysisError,
    analyze_category,
    ensure_utf8_console,
    load_json,
)

MIN_TOTAL_MISSION = 3
OUTPUT_FILE = os.path.join(os.path.dirname(DATA_FILE), "data_time_done.json")


ensure_utf8_console()


def get_valid_categories(data: Any, min_total: int) -> List[str]:
    """Return api_category values whose total_mission >= min_total."""
    categories: List[str] = []

    if not isinstance(data, list):
        return categories

    for group in data:
        if not isinstance(group, dict):
            continue

        total = group.get("total_mission")
        try:
            total_int = int(total)
        except (TypeError, ValueError):
            continue

        if total_int < min_total:
            continue

        api_category = group.get("api_category")
        if isinstance(api_category, str):
            categories.append(api_category)

    return categories


def print_summary_line(label: str, message: str) -> None:
    print(f"  {label}: {message}")


def main() -> None:
    data = load_json(DATA_FILE)
    categories = get_valid_categories(data, MIN_TOTAL_MISSION)
    results: List[dict] = []

    if not categories:
        print(f"Kông có api_category nào đạt total_mission >= {MIN_TOTAL_MISSION}.")
        return

    for api_category in categories:
        print(f"\n===== {api_category} =====")
        try:
            analysis = analyze_category(data, api_category)
        except CategoryAnalysisError as exc:
            print_summary_line("Lỗi", str(exc))
            continue

        iqr_stats = analysis["iqr_stats"]
        print_summary_line(
            "Thời gian TB",
            f"{analysis['average_time']:.0f} giây (lọc {FILTER_STRICTNESS})",
        )
        print_summary_line(
            "Dữ liệu",
            (
                f"gốc {int(iqr_stats['n_original'])} | "
                f"sau lọc {int(iqr_stats['n_filtered'])} | "
                f"loại {int(iqr_stats['n_removed'])}"
            ),
        )
        print_summary_line(
            "Khoảng IQR",
            f"{iqr_stats['lower']:.2f}s -> {iqr_stats['upper']:.2f}s",
        )
        results.append(
            {
                "api_category": api_category,
                "average_time": analysis["average_time"],
                "iqr_stats": iqr_stats,
            }
        )

    if results:
        with open(OUTPUT_FILE, "w", encoding="utf-8") as f:
            json.dump(results, f, ensure_ascii=False, indent=2)
        print(f"\nĐã lưu {len(results)} kết quả vào {OUTPUT_FILE}")
    else:
        print("\nKông có kết quả nào để lưu.")


if __name__ == "__main__":
    main()
