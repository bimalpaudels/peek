// Test peek with top-level await, concurrent promises, and async generators in Bun.

// 1. Top-Level Await with Simulated Async Latency
interface ApiResponse<T> {
  status: 200 | 404;
  data: T;
  latencyMs: number;
}

async function fetchUser(id: number): Promise<ApiResponse<{ name: string; email: string }>> {
  await Bun.sleep(15); // Native Bun micro-delay
  return {
    status: 200,
    data: { name: `User_${id}`, email: `user${id}@peek.dev` },
    latencyMs: 15,
  };
}

const userRes = await fetchUser(42);
userRes.data;

// 2. Concurrent Execution with Promise.all
async function fetchMetrics(metricName: string): Promise<{ metric: string; val: number }> {
  await Bun.sleep(10);
  return { metric: metricName, val: Math.round(Math.random() * 100) };
}

const metrics = await Promise.all([
  fetchMetrics("cpu"),
  fetchMetrics("memory"),
  fetchMetrics("network"),
]);
metrics.map((m) => `${m.metric}: ${m.val}`);

// 3. Resilient Batching with Promise.allSettled
const batchOperations = await Promise.allSettled([
  Promise.resolve({ task: "sync_cache", ok: true }),
  Promise.reject(new Error("Timeout connecting to database")),
  Promise.resolve({ task: "send_telemetry", ok: true }),
]);

batchOperations.map((r) => r.status);

// 4. Async Generator Stream
async function* countStream(max: number) {
  for (let i = 1; i <= max; i++) {
    await Bun.sleep(5);
    yield `packet #${i}`;
  }
}

const collectedPackets: string[] = [];
for await (const packet of countStream(4)) {
  collectedPackets.push(packet);
}
collectedPackets;

export {};
