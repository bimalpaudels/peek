# peek: Async Showcase
# Run whole file:
#   peek examples/03_async_showcase.py
# Or run specific lines:
#   peek examples/03_async_showcase.py:18
#   peek examples/03_async_showcase.py:28
#   peek examples/03_async_showcase.py:34

import asyncio


async def fetch_user(uid: int):
    await asyncio.sleep(0.01)
    return {"id": uid, "name": f"User_{uid}", "status": "active"}


# 1. Top-level await expression
await fetch_user(1)

# 2. Top-level await assignment
profile = await fetch_user(2)
profile

# 3. Auto-awaiting unawaited coroutine calls (IPython-style auto-await)
fetch_user(3)

# 4. Auto-awaiting concurrency primitives (asyncio.gather)
asyncio.gather(fetch_user(10), fetch_user(20), fetch_user(30))

# 5. Persistent event loop: Queue state shared across statements
queue = asyncio.Queue()
await queue.put({"msg": "message_in_queue", "priority": 1})
item = await queue.get()
item

# 6. Background tasks across statements
async def background_worker():
    await asyncio.sleep(0.01)
    return "background_task_completed"

task = asyncio.get_event_loop().create_task(background_worker())
res = await task
res

# 7. Asynchronous Comprehensions
async def number_stream():
    for n in [10, 20, 30]:
        await asyncio.sleep(0.005)
        yield n * 2

[x async for x in number_stream()]
