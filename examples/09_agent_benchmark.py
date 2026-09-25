# examples/09_agent_benchmark.py

raw_users = [
    {"id": 1, "name": "Alice", "score": 92, "active": True},
    {"id": 2, "name": "Bob", "score": 45, "active": False},
    {"id": 3, "name": "Charlie", "score": 88, "active": True},
    {"id": 4, "name": "Diana", "score": 76, "active": True},
    {"id": 5, "name": "Evan", "score": 59, "active": False},
]

# We want to inspect high_performers
high_performers = [
    {"name": u["name"], "rating": "A" if u["score"] >= 90 else "B"}
    for u in raw_users
    if u["active"] and u["score"] >= 75
]

# Calculate aggregate stats
total_score = sum(u["score"] for u in raw_users if u["active"])
average_active_score = round(total_score / len([u for u in raw_users if u["active"]]), 2)
