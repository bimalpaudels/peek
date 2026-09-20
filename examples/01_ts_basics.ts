// peek: TypeScript Basic Expressions, Stdout, Async & Object Formatting

// 1. Inline expression outputs (fits inline on the line)
const x: number = 42;
const y: number = x * 2;
y;

// 2. Captured stdout (block comments prefixed with ❯)
console.log(`Computed value y=${y}`);
for (let i = 0; i < 3; i++) {
  console.log(`item ${i}`);
}

// 3. Structured objects & interfaces
interface UserProfile {
  id: number;
  username: string;
  email: string;
  roles: string[];
  settings: { theme: string; notifications: boolean };
}

const userProfile: UserProfile = {
  id: 1001,
  username: "bimal",
  email: "bimal@example.com",
  roles: ["admin", "developer", "maintainer"],
  settings: { theme: "dark", notifications: true },
};
userProfile;

// 4. Top-level await
async function fetchScore(multiplier: number): Promise<number> {
  return 100 * multiplier;
}

await fetchScore(3);

// 5. Array transformations & methods
const numbers = Array.from({ length: 6 }, (_, i) => (i + 1) ** 2);
numbers;
