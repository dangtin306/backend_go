import requests
from bs4 import BeautifulSoup
import json
import html

HEADERS = {
    "User-Agent": "Mozilla/5.0"
}


# =========================
# LẤY LINK THEO PAGE
# =========================
def extract_product_links_by_page(page: int):
    if page == 1:
        url = "https://premium69.com/cua-hang/"
    else:
        url = f"https://premium69.com/cua-hang/page/{page}/"

    print(f"\n📄 Load page {page}: {url}")

    res = requests.get(url, headers=HEADERS, timeout=25)
    if res.status_code != 200:
        return []

    soup = BeautifulSoup(res.text, "html.parser")

    product_blocks = soup.select("div.product-small")
    if not product_blocks:
        return []

    results = []

    for block in product_blocks:
        a = block.select_one("a[aria-label][href]")
        if not a:
            continue

        href = a.get("href", "").strip()
        title = a.get("aria-label", "").strip()

        if not href or "/san-pham/" not in href:
            continue

        results.append({
            "title": title,
            "href": href
        })

    return results


# =========================
# FALLBACK – LẤY GIÁ HTML
# =========================
def extract_simple_price_from_html(soup):
    # meta JSON SEO price
    meta_price = soup.select_one('meta[itemprop="price"]')
    if meta_price and meta_price.get("content"):
        return int(meta_price["content"])

    og_price = soup.select_one('meta[property="product:price:amount"]')
    if og_price and og_price.get("content"):
        return int(og_price["content"])

    price_el = soup.select_one("p.price span.woocommerce-Price-amount bdi")
    if price_el:
        raw = price_el.get_text(strip=True)
        raw = (
            raw.replace("VNĐ", "")
            .replace("₫", "")
            .replace(".", "")
            .replace(",", "")
        )
        if raw.isdigit():
            return int(raw)

    return None


# =========================
# PARSE OFFER JSON
# =========================
def parse_price_from_offer(offer):
    sale_price = None
    original_price = None

    specs = offer.get("priceSpecification", [])
    if not isinstance(specs, list):
        specs = [specs]

    for spec in specs:
        price = spec.get("price")
        if not price:
            continue

        if spec.get("priceType", "").endswith("ListPrice"):
            original_price = price
        else:
            sale_price = price

    if not sale_price and original_price:
        sale_price = original_price
    if not original_price and sale_price:
        original_price = sale_price

    return original_price, sale_price


# =========================
# VARIABLE PRODUCT
# =========================
def extract_variations(soup):
    form = soup.find("form", class_="variations_form")
    if not form:
        return None

    raw = form.get("data-product_variations")
    if not raw:
        return None

    decoded = html.unescape(raw)

    try:
        variations = json.loads(decoded)
    except:
        return None

    results = []
    for v in variations:
        results.append({
            "type": "variation",
            "variation_id": v.get("variation_id"),
            "attributes": v.get("attributes"),
            "price": v.get("display_price"),
            "regular_price": v.get("display_regular_price"),
            "sku": v.get("sku"),
            "is_in_stock": v.get("is_in_stock"),
            "is_purchasable": v.get("is_purchasable"),
            "image": {
                "src": v.get("image", {}).get("full_src"),
                "thumb": v.get("image", {}).get("thumb_src"),
                "alt": v.get("image", {}).get("alt"),
            }
        })

    return results


# =========================
# SIMPLE PRODUCT / JSON-LD
# =========================
def extract_simple_json(soup):
    scripts = soup.find_all("script", type="application/ld+json")

    for script in scripts:
        try:
            raw = json.loads(script.string)

            if isinstance(raw, dict) and "@graph" in raw:
                items = raw["@graph"]
            elif isinstance(raw, list):
                items = raw
            else:
                items = [raw]

            for item in items:
                if item.get("@type") != "Product":
                    continue

                offers = item.get("offers")
                if not offers:
                    continue

                if isinstance(offers, dict):
                    offers = [offers]

                for offer in offers:
                    original_price, sale_price = parse_price_from_offer(offer)

                    return {
                        "type": "simple",
                        "title": item.get("name"),
                        "url": item.get("url"),
                        "image": item.get("image"),
                        "description": item.get("description"),
                        "price": int(sale_price) if sale_price else None,
                        "regular_price": int(original_price) if original_price else None,
                        "currency": "VND",
                        "brand": item.get("brand", {}).get("name")
                    }

        except:
            continue

    return None


# =========================
# MAIN PRODUCT PARSER
# =========================
def extract_product(url):
    res = requests.get(url, headers=HEADERS, timeout=25)
    soup = BeautifulSoup(res.text, "html.parser")

    # 1️⃣ thử variable trước
    variations = extract_variations(soup)
    if variations:
        print("➡️ Variable product detected")
        return variations

    # 2️⃣ thử JSON-LD
    json_product = extract_simple_json(soup)
    if json_product and json_product["price"]:
        print("➡️ Simple JSON-LD product detected")
        return [json_product]

    # 3️⃣ fallback HTML
    html_price = extract_simple_price_from_html(soup)
    if html_price:
        print("➡️ Simple HTML price detected")
        title_el = soup.select_one("h1.product-title")
        title = title_el.get_text(strip=True) if title_el else "Unknown"

        return [{
            "type": "simple",
            "title": title,
            "url": url,
            "price": html_price,
            "regular_price": html_price
        }]

    print("❌ No price detected")
    return []
