import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const protocolDir = path.join(root, "docs/protocol");
const manifest = JSON.parse(fs.readFileSync(path.join(protocolDir, "manifest.json"), "utf8"));
const openapi = fs.readFileSync(path.join(protocolDir, "openapi.yaml"), "utf8");
const wsSchema = JSON.parse(fs.readFileSync(path.join(protocolDir, "ws-events.schema.json"), "utf8"));
const routeContract = JSON.parse(fs.readFileSync(path.join(protocolDir, "routes.generated.json"), "utf8"));

const failures = [];

for (const requiredPath of manifest.requiredRestPaths) {
  if (!openapi.includes(requiredPath)) {
    failures.push(`openapi.yaml missing required path: ${requiredPath}`);
  }
  if (!routeContract.routes.some((route) => route.path === requiredPath)) {
    failures.push(`routes.generated.json missing required path: ${requiredPath}`);
  }
}

for (const field of manifest.requiredSendFields) {
  if (!openapi.includes(field) && !wsSchemaTextIncludes(field)) {
    failures.push(`protocol missing required send field: ${field}`);
  }
}

for (const field of manifest.requiredPublicIdFields ?? []) {
  if (!openapi.includes(field) && !wsSchemaTextIncludes(field)) {
    failures.push(`protocol missing required public_id field: ${field}`);
  }
}

const eventEnum = wsSchema?.properties?.type?.enum ?? [];
for (const eventName of manifest.requiredWebSocketEvents) {
  if (!eventEnum.includes(eventName)) {
    failures.push(`ws-events.schema.json missing event: ${eventName}`);
  }
}

if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}

console.log(`ANI protocol contract ${manifest.version} OK`);

function wsSchemaTextIncludes(text) {
  return JSON.stringify(wsSchema).includes(text);
}
