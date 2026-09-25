import { parse } from "@babel/parser";
import path from "node:path";
import crypto from "node:crypto";

const MUTATING_ARRAY_METHODS = new Set([
  "push",
  "pop",
  "shift",
  "unshift",
  "splice",
  "sort",
  "reverse",
  "fill",
  "copyWithin",
]);

const MUTATING_MAP_SET_METHODS = new Set([
  "set",
  "delete",
  "clear",
  "add",
]);

const MUTATING_METHODS = new Set([
  ...MUTATING_ARRAY_METHODS,
  ...MUTATING_MAP_SET_METHODS,
]);

const OUTPUT_PREFIXES = ["=>", "➜", "❯", "✕", "…"];

function isOutputLine(line: string): boolean {
  const s = line.trim();
  let rest = "";
  if (s.startsWith("//")) {
    rest = s.slice(2).trim();
  } else if (s.startsWith("#")) {
    rest = s.slice(1).trim();
  } else {
    return false;
  }
  return OUTPUT_PREFIXES.some((p) => rest.startsWith(p));
}

function hasOutputComment(line: string): boolean {
  const s = line.trim();
  if (!s) return false;
  if (isOutputLine(s)) return true;
  for (const sep of ["//", "#"]) {
    const idx = line.lastIndexOf(sep);
    if (idx !== -1) {
      const comment = line.slice(idx + sep.length).trim();
      if (OUTPUT_PREFIXES.some((p) => comment.startsWith(p))) {
        return true;
      }
    }
  }
  return false;
}

function getRootName(node: any): string | null {
  let curr = node;
  while (curr && (curr.type === "MemberExpression" || curr.type === "OptionalMemberExpression")) {
    curr = curr.object;
  }
  if (curr && curr.type === "Identifier") {
    return curr.name;
  }
  return null;
}

function getJsxRootName(node: any): string | null {
  let curr = node;
  while (curr && curr.type === "JSXMemberExpression") {
    curr = curr.object;
  }
  if (curr && curr.type === "JSXIdentifier") {
    return curr.name;
  }
  return null;
}

function extractJsxPragmas(source: string): { jsxPragma: string | null; jsxFragPragma: string | null } {
  let jsxPragma: string | null = null;
  let jsxFragPragma: string | null = null;
  const jsxMatch = source.match(/@jsx\s+([A-Za-z0-9_$.]+)/);
  if (jsxMatch) {
    jsxPragma = jsxMatch[1].split(".")[0];
  }
  const fragMatch = source.match(/@jsxFrag\s+([A-Za-z0-9_$.]+)/);
  if (fragMatch) {
    jsxFragPragma = fragMatch[1].split(".")[0];
  }
  return { jsxPragma, jsxFragPragma };
}

function checkJsx(
  sub: any,
  reads: Set<string>,
  localNames: Set<string> | null,
  jsxPragma: string | null,
  jsxFragPragma: string | null
) {
  if (sub.type === "JSXOpeningElement") {
    if (jsxPragma) {
      reads.add(jsxPragma);
    }
    let tagRoot: string | null = null;
    if (sub.name.type === "JSXIdentifier") {
      if (/^[A-Z]/.test(sub.name.name)) {
        tagRoot = sub.name.name;
      }
    } else if (sub.name.type === "JSXMemberExpression") {
      tagRoot = getJsxRootName(sub.name);
    }
    if (tagRoot && (!localNames || !localNames.has(tagRoot))) {
      reads.add(tagRoot);
    }
  } else if (sub.type === "JSXFragment" || sub.type === "JSXOpeningFragment") {
    if (jsxFragPragma) {
      reads.add(jsxFragPragma);
    }
  }
}

function extractBindingNames(pattern: any, defines: Set<string>) {
  if (!pattern) return;
  switch (pattern.type) {
    case "Identifier":
      defines.add(pattern.name);
      break;
    case "ObjectPattern":
      for (const prop of pattern.properties) {
        if (prop.type === "RestElement") {
          extractBindingNames(prop.argument, defines);
        } else if (prop.type === "ObjectProperty") {
          extractBindingNames(prop.value, defines);
        }
      }
      break;
    case "ArrayPattern":
      for (const el of pattern.elements) {
        if (el) {
          if (el.type === "RestElement") {
            extractBindingNames(el.argument, defines);
          } else {
            extractBindingNames(el, defines);
          }
        }
      }
      break;
    case "AssignmentPattern":
      extractBindingNames(pattern.left, defines);
      break;
    case "RestElement":
      extractBindingNames(pattern.argument, defines);
      break;
  }
}

function extractSymbols(
  node: any,
  jsxPragma: string | null = null,
  jsxFragPragma: string | null = null
): { defines: Set<string>; reads: Set<string>; is_side_effect: boolean } {
  const defines = new Set<string>();
  const reads = new Set<string>();
  let is_side_effect = false;

  // 1. Imports
  if (node.type === "ImportDeclaration") {
    if (!node.specifiers || node.specifiers.length === 0) {
      is_side_effect = true;
    } else {
      for (const spec of node.specifiers) {
        if (spec.local) {
          defines.add(spec.local.name);
        }
      }
    }
    return { defines, reads, is_side_effect };
  }

  // 2. Export declarations
  let effectiveNode = node;
  if (node.type === "ExportNamedDeclaration" && node.declaration) {
    effectiveNode = node.declaration;
  } else if (node.type === "ExportDefaultDeclaration" && node.declaration) {
    effectiveNode = node.declaration;
  }

  // 3. Functions
  if (effectiveNode.type === "FunctionDeclaration") {
    if (effectiveNode.id) {
      defines.add(effectiveNode.id.name);
    }
    const localNames = new Set<string>();
    for (const param of effectiveNode.params) {
      extractBindingNames(param, localNames);
    }
    walkAst(effectiveNode.body, (sub, parent) => {
      if (sub.type === "VariableDeclarator") {
        extractBindingNames(sub.id, localNames);
      }
    });
    walkAst(effectiveNode.body, (sub, parent) => {
      if (sub.type === "Identifier" && !localNames.has(sub.name)) {
        if (isReadContext(sub, parent)) {
          reads.add(sub.name);
        }
      }
      checkJsx(sub, reads, localNames, jsxPragma, jsxFragPragma);
    });
    return { defines, reads, is_side_effect };
  }

  // 4. Classes
  if (effectiveNode.type === "ClassDeclaration") {
    if (effectiveNode.id) {
      defines.add(effectiveNode.id.name);
    }
    if (effectiveNode.superClass && effectiveNode.superClass.type === "Identifier") {
      reads.add(effectiveNode.superClass.name);
    }
    walkAst(effectiveNode.body, (sub, parent) => {
      if (sub.type === "Identifier") {
        if (isReadContext(sub, parent)) {
          reads.add(sub.name);
        }
      }
      checkJsx(sub, reads, null, jsxPragma, jsxFragPragma);
    });
    return { defines, reads, is_side_effect };
  }

  // 5. TypeScript Enums
  if (effectiveNode.type === "TSEnumDeclaration") {
    if (effectiveNode.id) {
      defines.add(effectiveNode.id.name);
    }
    return { defines, reads, is_side_effect };
  }

  // 6. Variable Declarations
  if (effectiveNode.type === "VariableDeclaration") {
    for (const decl of effectiveNode.declarations) {
      extractBindingNames(decl.id, defines);
      if (decl.init) {
        walkAst(decl.init, (sub, parent) => {
          checkMutations(sub, defines, reads);
          if (sub.type === "Identifier" && isReadContext(sub, parent)) {
            reads.add(sub.name);
          }
          checkJsx(sub, reads, null, jsxPragma, jsxFragPragma);
        });
      }
    }
    return { defines, reads, is_side_effect };
  }

  // 7. General Statements / Expressions / Mutations
  walkAst(effectiveNode, (sub, parent) => {
    checkMutations(sub, defines, reads);
    if (sub.type === "Identifier" && isReadContext(sub, parent)) {
      reads.add(sub.name);
    }
    checkJsx(sub, reads, null, jsxPragma, jsxFragPragma);
  });

  return { defines, reads, is_side_effect };
}

function checkMutations(sub: any, defines: Set<string>, reads: Set<string>) {
  if (sub.type === "AssignmentExpression") {
    const root = getRootName(sub.left);
    if (root) {
      defines.add(root);
      reads.add(root);
    }
  } else if (sub.type === "UpdateExpression") {
    const root = getRootName(sub.argument);
    if (root) {
      defines.add(root);
      reads.add(root);
    }
  } else if (
    sub.type === "CallExpression" &&
    sub.callee &&
    (sub.callee.type === "MemberExpression" || sub.callee.type === "OptionalMemberExpression")
  ) {
    const prop = sub.callee.property;
    let methodName: string | null = null;
    if (!sub.callee.computed && prop && prop.type === "Identifier") {
      methodName = prop.name;
    } else if (sub.callee.computed && prop && prop.type === "StringLiteral") {
      methodName = prop.value;
    }

    if (methodName && MUTATING_METHODS.has(methodName)) {
      const root = getRootName(sub.callee.object);
      if (root) {
        defines.add(root);
        reads.add(root);
      }
    }
  }
}

function isReadContext(node: any, parent: any): boolean {
  if (!parent) return true;
  // Ignore property keys in object literals: { foo: 1 }
  if (parent.type === "ObjectProperty" && parent.key === node && !parent.computed) {
    return false;
  }
  // Ignore property keys in member expressions: obj.foo
  if ((parent.type === "MemberExpression" || parent.type === "OptionalMemberExpression") && parent.property === node && !parent.computed) {
    return false;
  }
  // Ignore JSX attribute names: <div className="..." /> -> className is not a variable read
  if (parent.type === "JSXAttribute" && parent.name === node) {
    return false;
  }
  // Ignore type annotations
  if (
    parent.type.startsWith("TS") ||
    parent.type === "TSTypeAnnotation" ||
    parent.type === "TSTypeReference"
  ) {
    return false;
  }
  return true;
}

function walkAst(node: any, cb: (n: any, parent: any) => void, parent: any = null) {
  if (!node || typeof node !== "object") return;
  cb(node, parent);
  for (const key of Object.keys(node)) {
    if (key === "parent" || key === "loc") continue;
    const val = node[key];
    if (Array.isArray(val)) {
      for (const child of val) {
        if (child && typeof child === "object" && child.type) {
          walkAst(child, cb, node);
        }
      }
    } else if (val && typeof val === "object" && val.type) {
      walkAst(val, cb, node);
    }
  }
}

function computeDependencies(statements: any[], targetIdx: number): number[] {
  const needed = new Set<number>([targetIdx]);
  const unresolvedReads = new Set<string>(statements[targetIdx].reads);

  let changed = true;
  while (changed) {
    changed = false;
    for (let idx = targetIdx - 1; idx >= 0; idx--) {
      const stmt = statements[idx];
      if (stmt.is_side_effect && !needed.has(idx)) {
        needed.add(idx);
        for (const r of stmt.reads) unresolvedReads.add(r);
        changed = true;
        continue;
      }
      let intersects = false;
      for (const d of stmt.defines) {
        if (unresolvedReads.has(d)) {
          intersects = true;
          break;
        }
      }
      if (intersects && !needed.has(idx)) {
        needed.add(idx);
        for (const r of stmt.reads) unresolvedReads.add(r);
        changed = true;
      }
    }
  }
  return Array.from(needed).sort((a, b) => a - b);
}

function formatValue(val: any, maxStrLen = 140): string | null {
  if (val === undefined) return null;
  if (typeof val === "string") {
    if (val.length > maxStrLen) {
      return JSON.stringify(val.slice(0, maxStrLen) + "…");
    }
    return JSON.stringify(val);
  }
  return Bun.inspect(val, { depth: 4, colors: false });
}

function formatOutputs(stdLines: string[], resultRepr: string | null, errorLines: string[], maxLines: number): string[] {
  const raw: string[] = [];
  for (const l of stdLines) {
    raw.push(`❯ ${l}`);
  }
  if (resultRepr !== null && resultRepr !== undefined) {
    for (const l of resultRepr.split("\n")) {
      raw.push(`➜ ${l}`);
    }
  }
  for (const el of errorLines) {
    raw.push(`✕ ${el}`);
  }
  if (raw.length > maxLines) {
    return [...raw.slice(0, maxLines), `… [truncated: ${raw.length - maxLines} lines hidden]`];
  }
  return raw;
}

async function run() {
  const inputChunks: Buffer[] = [];
  for await (const chunk of process.stdin) {
    inputChunks.push(Buffer.from(chunk));
  }
  const rawInput = Buffer.concat(inputChunks).toString("utf-8");

  let payload: any = {};
  try {
    payload = JSON.parse(rawInput);
  } catch (e: any) {
    console.log(JSON.stringify({ error: `Failed to parse payload: ${e}` }));
    return;
  }

  const filePath = payload.file_path || "<scratchpad>";
  const source = payload.source || "";
  const targetLine = payload.target_line;
  const maxLines = payload.max_lines || 30;
  const maxStrLen = payload.max_str_len || 140;

  if (!source.trim()) {
    console.log(JSON.stringify({ blocks: [] }));
    return;
  }

  let ast: any;
  try {
    ast = parse(source, {
      sourceType: "module",
      plugins: [
        "typescript",
        "jsx",
        "decorators-legacy",
        "topLevelAwait",
        "classProperties",
        "classPrivateProperties",
        "classPrivateMethods",
        "exportDefaultFrom",
      ],
      tokens: false,
    });
  } catch (err: any) {
    const line = err.loc?.line || 1;
    const col = (err.loc?.column ?? 0) + 1;
    const msg = err.message || "syntax error";
    console.log(
      JSON.stringify({
        syntax_error: {
          line,
          col,
          msg: `SyntaxError at line ${line}, col ${col}: ${msg}`,
        },
        blocks: [],
      })
    );
    return;
  }

  if (!ast.program || !ast.program.body || ast.program.body.length === 0) {
    console.log(JSON.stringify({ blocks: [] }));
    return;
  }

  const sourceLines = source.split(/\r?\n/);
  const statements: any[] = [];
  const { jsxPragma, jsxFragPragma } = extractJsxPragmas(source);

  for (let idx = 0; idx < ast.program.body.length; idx++) {
    const node = ast.program.body[idx];
    const startLn = node.loc ? node.loc.start.line : 1;
    let endLn = node.loc ? node.loc.end.line : startLn;

    while (endLn < sourceLines.length && isOutputLine(sourceLines[endLn])) {
      endLn++;
    }

    const { defines, reads, is_side_effect } = extractSymbols(node, jsxPragma, jsxFragPragma);
    statements.push({
      index: idx,
      start_line: startLn,
      end_line: endLn,
      node,
      defines,
      reads,
      is_side_effect,
    });
  }

  let targetIdx: number | null = null;
  if (targetLine !== null && targetLine !== undefined) {
    for (let idx = 0; idx < statements.length; idx++) {
      const stmt = statements[idx];
      if (stmt.start_line <= targetLine && targetLine <= stmt.end_line) {
        targetIdx = idx;
        break;
      }
    }

    // Blank line or comment: strict no-op
    if (targetIdx === null) {
      console.log(JSON.stringify({ blocks: [] }));
      return;
    }

    // Inert declarations: strict no-op
    const targetNode = statements[targetIdx].node;
    let effectiveTarget = targetNode;
    if (targetNode.type === "ExportNamedDeclaration" && targetNode.declaration) {
      effectiveTarget = targetNode.declaration;
    }
    if (
      effectiveTarget.type === "FunctionDeclaration" ||
      effectiveTarget.type === "ClassDeclaration" ||
      effectiveTarget.type === "TSInterfaceDeclaration" ||
      effectiveTarget.type === "TSTypeAliasDeclaration" ||
      (targetNode.type === "ExportNamedDeclaration" && !targetNode.declaration) ||
      targetNode.type === "ExportAllDeclaration"
    ) {
      console.log(JSON.stringify({ blocks: [] }));
      return;
    }
  }

  let neededIndices = new Set<number>(statements.map((s) => s.index));
  if (targetIdx !== null) {
    neededIndices = new Set<number>(computeDependencies(statements, targetIdx));
  }

  const workDir = filePath !== "<scratchpad>" ? path.dirname(path.resolve(filePath)) : process.cwd();

  // Synthesize runner script
  const topImports: string[] = [];
  const bodyStatements: string[] = [];

  for (const stmt of statements) {
    if (!neededIndices.has(stmt.index)) continue;
    const node = stmt.node;

    if (node.type === "ImportDeclaration") {
      let importCode = source.slice(node.start, node.end);
      if (node.source && typeof node.source.value === "string") {
        try {
          const resolved = Bun.resolveSync(node.source.value, workDir);
          const relStart = node.source.start - node.start;
          const relEnd = node.source.end - node.start;
          importCode = importCode.slice(0, relStart) + JSON.stringify(resolved) + importCode.slice(relEnd);
        } catch {
          // Keep original specifier so it errors naturally during execution
        }
      }
      topImports.push(importCode);
      continue;
    }

    if (node.type === "ExportNamedDeclaration" && !node.declaration) {
      // Bare export statements (e.g. export {}; or export { a };) cannot be placed inside try/catch blocks
      continue;
    }

    if (node.type === "ExportAllDeclaration") {
      continue;
    }

    let code = source.slice(node.start, node.end);
    let effectiveNode = node;
    let isExpr = node.type === "ExpressionStatement";

    if (node.type === "ExportNamedDeclaration" && node.declaration) {
      effectiveNode = node.declaration;
      code = source.slice(node.declaration.start, node.declaration.end);
      isExpr = node.declaration.type === "ExpressionStatement";
    } else if (node.type === "ExportDefaultDeclaration" && node.declaration) {
      effectiveNode = node.declaration;
      code = source.slice(node.declaration.start, node.declaration.end);
      isExpr = true;
    }

    const isVarDecl = effectiveNode.type === "VariableDeclaration";
    const varDeclCandidates = isVarDecl
      ? Array.from(stmt.defines).filter((name: any) => typeof name === "string" && !name.startsWith("_"))
      : [];

    if (isExpr) {
      const exprNode = (node.type === "ExpressionStatement") ? node.expression : node.declaration;
      const exprCode = source.slice(exprNode.start, exprNode.end);
      bodyStatements.push(`
__peek_before__(${stmt.index});
const __res_${stmt.index}__ = await (${exprCode});
__peek_after__(${stmt.index}, __res_${stmt.index}__);
`);
    } else if (isVarDecl) {
      let captureExpr = "undefined";
      if (varDeclCandidates.length === 1) {
        captureExpr = String(varDeclCandidates[0]);
      } else if (varDeclCandidates.length > 1) {
        captureExpr = `{ ${varDeclCandidates.join(", ")} }`;
      }
      bodyStatements.push(`
__peek_before__(${stmt.index});
${code};
__peek_after__(${stmt.index}, ${captureExpr});
`);
    } else {
      bodyStatements.push(`
__peek_before__(${stmt.index});
${code};
__peek_after__(${stmt.index}, undefined);
`);
    }
  }

  const runnerScript = `
globalThis.__peek_records__ = {};
let __current_idx__ = -1;
let __logs__ = [];
const origLog = console.log;
const origErr = console.error;
const origWarn = console.warn;
const origInfo = console.info;

function __peek_capture__(...args) {
  const str = args.map(a => typeof a === 'string' ? a : Bun.inspect(a, { depth: 3, colors: false })).join(' ');
  __logs__.push(str);
}

function __peek_before__(idx) {
  __current_idx__ = idx;
  __logs__ = [];
}

function __peek_after__(idx, val) {
  globalThis.__peek_records__[idx] = { logs: [...__logs__], val };
}

${topImports.join("\n")}

console.log = __peek_capture__;
console.error = __peek_capture__;
console.warn = __peek_capture__;
console.info = __peek_capture__;

try {
  ${bodyStatements.join("\n")}
} catch (err) {
  globalThis.__peek_records__[__current_idx__] = { logs: [...__logs__], err };
} finally {
  console.log = origLog;
  console.error = origErr;
  console.warn = origWarn;
  console.info = origInfo;
}
`;

  const ext = path.extname(filePath).toLowerCase();
  const loader = (ext === ".tsx" || ext === ".jsx") ? "tsx" : (ext === ".js" || ext === ".mjs" || ext === ".cjs") ? "js" : "ts";
  const rand = crypto.randomBytes(6).toString("hex");
  const virtualModuleId = `peek:run:${Date.now()}_${rand}`;

  Bun.plugin({
    setup(builder) {
      builder.module(virtualModuleId, () => ({
        contents: runnerScript,
        loader,
      }));
    },
  });

  let executionRecords: Record<number, any> = {};
  try {
    await import(virtualModuleId);
    executionRecords = (globalThis as any).__peek_records__ || {};
  } catch (importErr: any) {
    console.log(JSON.stringify({
      error: `Execution initialization failed: ${importErr?.message || importErr}`,
      blocks: [],
    }));
    return;
  }

  const results: any[] = [];

  for (const stmt of statements) {
    if (!neededIndices.has(stmt.index)) continue;
    const rec = executionRecords[stmt.index] || { logs: [], val: undefined };

    // Upstream error in sliced execution
    if (targetIdx !== null && stmt.index < targetIdx) {
      if (rec.err) {
        const errMsg = rec.err.message || String(rec.err);
        console.log(
          JSON.stringify({
            error: `Upstream statement at line ${stmt.start_line} error: ${errMsg}`,
            blocks: [],
          })
        );
        return;
      }
      continue;
    }

    const resultRepr = formatValue(rec.val, maxStrLen);
    const errorLines: string[] = [];
    if (rec.err) {
      errorLines.push(`${rec.err.name || "Error"}: ${rec.err.message || String(rec.err)}`);
      if (rec.err.stack) {
        const stackLines = rec.err.stack.split("\n").slice(1);
        for (const l of stackLines) {
          if (l.includes(".peek_tmp_") || l.includes("harness.js") || l.includes("harness.ts") || l.includes("peek:run:")) {
            continue;
          }
          errorLines.push(l.trim());
        }
      }
    }

    const origEnd = stmt.node.loc ? stmt.node.loc.end.line : stmt.start_line;
    const hasExistingOutputs =
      stmt.end_line > origEnd ||
      Array.from({ length: stmt.end_line - stmt.start_line + 1 }, (_, i) => stmt.start_line + i).some((ln) => {
        const lineIdx = ln - 1;
        return lineIdx < sourceLines.length && hasOutputComment(sourceLines[lineIdx]);
      });

    const isTarget = targetIdx !== null && stmt.index === targetIdx;
    let effective = stmt.node;
    if (stmt.node.type === "ExportNamedDeclaration" && stmt.node.declaration) {
      effective = stmt.node.declaration;
    }
    const isVarDecl = effective.type === "VariableDeclaration";

    let outputs = formatOutputs(rec.logs, resultRepr, errorLines, maxLines);
    if (isVarDecl && !isTarget && !hasExistingOutputs) {
      outputs = [];
    }

    if (targetIdx !== null || outputs.length > 0 || hasExistingOutputs) {
      results.push({
        start_line: stmt.start_line,
        end_line: stmt.end_line,
        outputs,
      });
      if (targetIdx !== null) {
        break;
      }
    }

    if (errorLines.length > 0) {
      break;
    }
  }

  console.log(JSON.stringify({ blocks: results }));
}

run().catch((e) => {
  console.log(JSON.stringify({ error: `Harness failure: ${e}` }));
});
