# peek: Data Transformation & Aggregation Pipeline
# Run:
#   peek examples/05_pipeline.py
# Or evaluate specific stages:
#   peek examples/05_pipeline.py:33
#   peek examples/05_pipeline.py:55
#   peek examples/05_pipeline.py:74

import json
import urllib.request
from collections import defaultdict


# 1. Extraction: Fetch from public API (with offline fallback)
def fetch_data(limit=10):
    try:
        url = f"https://dummyjson.com/products?limit={limit}"
        req = urllib.request.Request(url, headers={"User-Agent": "peek-demo/1.0"})
        with urllib.request.urlopen(req, timeout=5) as resp:
            return json.loads(resp.read().decode("utf-8"))["products"]
    except Exception:
        # Offline mock data fallback
        return [
            {"id": 1, "title": "Wireless Earbuds", "category": "electronics", "price": 99.99, "discountPercentage": 15.0, "stock": 45, "rating": 4.6},
            {"id": 2, "title": "Mechanical Keyboard", "category": "electronics", "price": 149.50, "discountPercentage": 10.0, "stock": 20, "rating": 4.8},
            {"id": 3, "title": "Ceramic Coffee Mug", "category": "home", "price": 18.00, "discountPercentage": 5.0, "stock": 100, "rating": 4.3},
            {"id": 4, "title": "Ergonomic Desk Mat", "category": "home", "price": 29.99, "discountPercentage": 20.0, "stock": 35, "rating": 4.7},
            {"id": 5, "title": "Smart Watch", "category": "electronics", "price": 199.99, "discountPercentage": 12.5, "stock": 15, "rating": 4.2},
        ]


raw_products = fetch_data()
raw_products[0]


# 2. Transformation: Clean, compute discounted pricing & inventory values
def transform_product(p):
    price = p["price"]
    discount = p.get("discountPercentage", 0.0)
    final_price = round(price * (1 - discount / 100), 2)
    return {
        "id": p["id"],
        "title": p["title"],
        "category": p["category"],
        "original_price": price,
        "discount_pct": discount,
        "price": final_price,
        "stock": p["stock"],
        "inventory_value": round(final_price * p["stock"], 2),
        "rating": p["rating"],
    }


cleaned = [transform_product(p) for p in raw_products]
cleaned[0]


# 3. Aggregation: Category-level volume & revenue metrics
categories = defaultdict(lambda: {"count": 0, "total_value": 0.0, "ratings": []})
for item in cleaned:
    cat = categories[item["category"]]
    cat["count"] += 1
    cat["total_value"] = round(cat["total_value"] + item["inventory_value"], 2)
    cat["ratings"].append(item["rating"])

category_summary = {
    cat: {
        "items": stats["count"],
        "total_value": stats["total_value"],
        "avg_rating": round(sum(stats["ratings"]) / len(stats["ratings"]), 2),
    }
    for cat, stats in categories.items()
}
category_summary


# 4. Slicing & Filter: Top-rated high-performing products
high_rated = sorted(
    [p for p in cleaned if p["rating"] >= 4.5],
    key=lambda x: x["rating"],
    reverse=True,
)
[(p["title"], p["rating"], f"${p['price']}") for p in high_rated[:3]]


# 5. Output Reporting
print("=== PIPELINE RUN REPORT ===")
print(f"Total Products Processed: {len(cleaned)}")
print(f"Total Portfolio Value: ${sum(p['inventory_value'] for p in cleaned):,.2f}")
print(f"Categories Analyzed: {len(category_summary)}")
