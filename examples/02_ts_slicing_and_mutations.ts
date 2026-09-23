// Test peek's program slicing, dependency tracking, and in-place collection mutations.

// --- Branch A: Heavy / Unrelated Setup ---
// If you evaluate lines in Branch B or C, Branch A will be completely skipped!
const heavyList = Array.from({ length: 50 }, (_, i) => ({ id: i, hash: (i * 7919).toString(16) }));
console.log("Branch A: Heavy computation finished!");

// heavyList and its console.log are skipped because line 13 only needs fastValue!
const fastValue = (12 * 8) + 3;
fastValue;

// --- Branch B: Array Mutations ---
// peek tracks mutating methods (.push, .splice, .sort, .reverse) and slices them in automatically!
const languages: string[] = ["Rust", "Go"];
languages.push("TypeScript");
languages.push("Zig");
languages.splice(1, 0, "Python");

// Notice: languages shows all mutations applied in order!
languages;

// --- Branch C: Map & Set In-Place Mutations ---
const cache = new Map<string, number>();
cache.set("cpu_usage", 42);
cache.set("mem_usage", 68);
cache.set("disk_io", 15);

cache.get("mem_usage");
cache.size;

const tags = new Set<string>(["cli", "fast"]);
tags.add("terminal");
tags.add("minimal");

Array.from(tags);
