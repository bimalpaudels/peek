# peek: Basic Expressions, Stdout, Errors & Pretty Printing

# 1. Inline expression outputs (fits inline on the line)
x = 42
y = x * 2
y

# 2. Captured stdout (block comments prefixed with ❯)
print(f"Computed value y={y}")
for i in range(3):
    print(f"item {i}")

# 3. Compact 1-line-per-key formatting for dictionaries
user_profile = {
    "id": 1001,
    "username": "bimal",
    "email": "bimal@example.com",
    "roles": ["admin", "developer", "maintainer"],
    "settings": {"theme": "dark", "notifications": True},
}
user_profile

# 4. Long collections (automatically wrapped cleanly)
numbers = [i ** 2 for i in range(15)]
numbers
