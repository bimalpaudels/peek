# peek: Program Slicing & Dependency Tracking

# --- Branch A (Slow setup) ---
import time
heavy_data = [i for i in range(100)]
print("Heavy setup completed!")

# Target this line: peek examples/02_slicing.py:11
# Notice: 'heavy_data' is NOT evaluated because line 11 only needs 'fast_val'!
fast_val = 100 + 23
fast_val

# --- Branch B (Object Mutations) ---
items = ["apple", "banana"]
items.append("cherry")
items.insert(0, "avocado")

# Target this line: peek examples/02_slicing.py:20
# Notice: peek automatically runs lines 14-16 before line 20, but skips Branch A!
items
