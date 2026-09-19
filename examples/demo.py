# Basic arithmetic and variables
x = [1, 2, 3, 4]
y = [i * 10 for i in x]
y

# Persistence: using variables from previous block
print(f"Sum of y is: {sum(y)}")
total = sum(y) + 50
total


# Functions and algorithms
def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        diff = target - n
        if diff in seen:
            return [seen[diff], i]
        seen[n] = i

two_sum([2, 7, 11, 15], 9)  # ➜ [0, 1]
