# peek: Algorithms, Dataclasses & Scratchpad Tricks
# Run whole file:
#   peek examples/06_algorithms_and_tricks.py
# Or run specific lines:
#   peek examples/06_algorithms_and_tricks.py:22
#   peek examples/06_algorithms_and_tricks.py:35
#   peek examples/06_algorithms_and_tricks.py:58

from dataclasses import dataclass


# 1. Classic Two-Sum Algorithm
def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        diff = target - n
        if diff in seen:
            return [seen[diff], i]
        seen[n] = i


two_sum([2, 7, 11, 15], 9)
two_sum([3, 2, 4], 6)


# 2. Maximum Unique Subarray Length
garbage_list = [1, 4, 5, 1, 5, 6, 7, 1]

sub_lists = []
for i in range(len(garbage_list)):
    for j in range(i + 1, len(garbage_list) + 1):
        sub_lists.append(garbage_list[i:j])

max_unique_len = max(len(set(sub)) for sub in sub_lists)
max_unique_len


# 3. Python Dataclasses (automatically normalized and pretty-printed)
@dataclass
class Point:
    x: float
    y: float
    label: str


@dataclass
class Route:
    origin: Point
    destination: Point
    distance_km: float


route = Route(
    origin=Point(37.7749, -122.4194, "San Francisco"),
    destination=Point(34.0522, -118.2437, "Los Angeles"),
    distance_km=615.4,
)
route


# 4. Comprehensions & Cumulative Sums
nums = [1, 2, 3, 4, 5]
squares = [n * n for n in nums]
print(f"Squares: {squares}, Total: {sum(squares)}")
