import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const routerPath = path.join(root, "internal/handler/router.go");
const outputPath = path.join(root, "docs/protocol/routes.generated.json");
const check = process.argv.includes("--check");

const source = fs.readFileSync(routerPath, "utf8");
const routes = extractRoutes(source);
const payload = {
  generatedFrom: "internal/handler/router.go",
  routeCount: routes.length,
  routes,
};
const rendered = `${JSON.stringify(payload, null, 2)}\n`;

if (check) {
  const current = fs.existsSync(outputPath) ? fs.readFileSync(outputPath, "utf8") : "";
  if (current !== rendered) {
    console.error("docs/protocol/routes.generated.json is out of date.");
    console.error("Run: node scripts/generate-route-contract.mjs");
    process.exit(1);
  }
  console.log(`ANI route contract OK (${routes.length} routes)`);
} else {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, rendered);
  console.log(`wrote ${outputPath} (${routes.length} routes)`);
}

function extractRoutes(text) {
  const routePattern = /\b(v1|authed|full|admin)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]+)"/g;
  const seen = new Set();
  const routes = [];
  let match;

  while ((match = routePattern.exec(text))) {
    const method = match[2];
    const rawPath = match[3].startsWith("/") ? match[3] : `/${match[3]}`;
    const routePath = normalizePath(`/api/v1${rawPath}`);
    const key = `${method} ${routePath}`;
    if (seen.has(key)) continue;
    seen.add(key);
    routes.push({ method, path: routePath });
  }

  routes.sort((a, b) => `${a.method} ${a.path}`.localeCompare(`${b.method} ${b.path}`));
  return routes;
}

function normalizePath(value) {
  return value
    .replace(/:([A-Za-z0-9_]+)/g, "{$1}")
    .replace(/\*([A-Za-z0-9_]+)/g, "{$1}");
}

