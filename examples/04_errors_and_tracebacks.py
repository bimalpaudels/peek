# peek: Error Handling & Traceback Hygiene
# Run:
#   peek examples/04_errors_and_tracebacks.py:11
#   peek examples/04_errors_and_tracebacks.py:21

import asyncio

# 1. Simple synchronous division error
# Target: peek examples/04_errors_and_tracebacks.py:11
divisor = 0
100 / divisor

# 2. Async error with clean traceback
# Target: peek examples/04_errors_and_tracebacks.py:21
# Notice: 'asyncio/runners.py' and 'asyncio/base_events.py' are stripped,
# pointing directly to 'fail_async()' and the target line!
async def fail_async():
    await asyncio.sleep(0.01)
    raise KeyError("missing_key_in_db")

await fail_async()
