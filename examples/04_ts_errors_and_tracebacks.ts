// Test error capture, clean stack traces, and custom error classes in peek.

// 1. Synchronous Exception

JSON.parse("{ invalid json syntax }");

// 2. Custom Domain Error
class ValidationError extends Error {
  constructor(public field: string, message: string) {
    super(`Validation failed for field "${field}": ${message}`);
    this.name = "ValidationError";
  }
}

function validateEmail(email: string) {
  if (!email.includes("@")) {
    throw new ValidationError("email", "must contain '@'");
  }
  return email.toLowerCase();
}

validateEmail("not-an-email");

validateEmail("dev@peek.sh");

// 3. Async Rejection in Promise / Async Function
async function fetchAccount(id: number) {
  await Bun.sleep(10);
  if (id <= 0) {
    throw new RangeError(`Account ID must be positive, got ${id}`);
  }
  return { id, name: "Alice" };
}

await fetchAccount(-5);

export {};
