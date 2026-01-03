import json
import time
from crawlData import extract_product_links_by_page, extract_product

if __name__ == "__main__":
    all_products = []
    seen_links = set()
    page = 1

    while True:
        links = extract_product_links_by_page(page)

        if not links:
            print("\n🚫 HẾT PAGE – DỪNG")
            break

        new_links = 0

        for link in links:
            if link["href"] in seen_links:
                continue

            seen_links.add(link["href"])
            new_links += 1

            print(f"\n🔍 Crawl product: {link['href']}")
            products = extract_product(link["href"])
            all_products.extend(products)

            print(json.dumps(products, ensure_ascii=False, indent=2))

            time.sleep(1)

        if new_links == 0:
            print("\n🚫 Không còn sản phẩm mới – Dừng")
            break
        # =========================
        #  LƯU FILE JSON SITE
        # =========================
        print("\n💾 Đang lưu products.json ...")

        with open("products.json", "w", encoding="utf-8") as f:
            json.dump(all_products, f, ensure_ascii=False, indent=2)

        print(f"✅ Done! Tổng sản phẩm: {len(all_products)}")
        print("📂 File: products.json")

        page += 1


