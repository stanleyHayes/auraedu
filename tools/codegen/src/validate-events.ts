#!/usr/bin/env node
/** Validate every event contract as JSON Schema plus an AuraEDU CloudEvent envelope. */
import { promises as fs } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import Ajv, { type AnySchema } from "ajv";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const eventsDirectory = path.join(root, "contracts", "events");
const appsDirectory = path.join(root, "apps");
const draft7 = new Ajv({ allErrors: true, strict: false });
const draft2020 = new Ajv2020({ allErrors: true, strict: false });
addFormats(draft7);
addFormats(draft2020);

function assertContract(condition: unknown, file: string, message: string): asserts condition {
  if (!condition) throw new Error(`${file}: ${message}`);
}

export function directProducerEventTypes(contents: string): string[] {
  return [
    ...new Set(
      [...contents.matchAll(/NewCloudEvent\(\s*["']([a-z][a-z0-9_-]*(?:\.[a-z0-9_-]+)+)["']/g)]
        .map((match) => match[1])
        .filter((eventType): eventType is string => Boolean(eventType)),
    ),
  ];
}

export function consumerEventCandidates(contents: string): string[] {
  if (!contents.includes("eventbus.Subscribe") && !contents.includes("SUBSCRIBED_EVENTS"))
    return [];
  return [
    ...new Set(
      [...contents.matchAll(/["']([a-z][a-z0-9_-]*(?:\.[a-z0-9_-]+)+)["']/g)]
        .map((match) => match[1])
        .filter((eventType): eventType is string => Boolean(eventType)),
    ),
  ];
}

export function runtimeEventFailures(
  contents: string,
  location: string,
  eventTypes: ReadonlySet<string>,
): string[] {
  const failures: string[] = [];
  if (/NewCloudEvent\(\s*strings\.TrimSuffix\([^,]+,\s*["']\.v1["']\)/.test(contents)) {
    failures.push(
      `${location}: producer strips the contract version before constructing a CloudEvent`,
    );
  }
  for (const eventType of directProducerEventTypes(contents)) {
    if (!eventTypes.has(eventType)) {
      const versioned = `${eventType}.v1`;
      failures.push(
        eventTypes.has(versioned)
          ? `${location}: producer emits unversioned ${eventType}; use contract type ${versioned}`
          : `${location}: producer emits ${eventType} without an exact event contract`,
      );
    }
  }
  for (const eventType of consumerEventCandidates(contents)) {
    if (eventTypes.has(eventType)) continue;
    const versioned = `${eventType}.v1`;
    if (eventTypes.has(versioned)) {
      failures.push(
        `${location}: consumer subscribes to unversioned ${eventType}; use contract type ${versioned}`,
      );
    }
  }
  return failures;
}

export function producerCoverageFailures(
  producerServices: ReadonlySet<string>,
  schemaBackedServices: ReadonlySet<string>,
): string[] {
  return [...producerServices]
    .filter((service) => !schemaBackedServices.has(service))
    .sort()
    .map(
      (service) =>
        `apps/${service}: production CloudEvent constructor has no schema-backed AssertEventContract test`,
    );
}

export function pythonProducerCoverageFailures(
  producerServices: ReadonlySet<string>,
  schemaBackedServices: ReadonlySet<string>,
): string[] {
  return [...producerServices]
    .filter((service) => !schemaBackedServices.has(service))
    .sort()
    .map(
      (service) =>
        `apps/${service}: production Python event encoder has no schema-backed assert_event_contract test`,
    );
}

async function productionSources(directory: string): Promise<string[]> {
  const sources: string[] = [];
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (!["node_modules", ".next", "dist", "test", "tests"].includes(entry.name)) {
        sources.push(...(await productionSources(absolute)));
      }
    } else if (
      entry.isFile() &&
      (entry.name.endsWith(".go") || entry.name.endsWith(".py")) &&
      !entry.name.endsWith("_test.go") &&
      !entry.name.startsWith("test_")
    ) {
      sources.push(absolute);
    }
  }
  return sources;
}

async function goTestSources(directory: string): Promise<string[]> {
  const sources: string[] = [];
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (!["node_modules", ".next", "dist"].includes(entry.name)) {
        sources.push(...(await goTestSources(absolute)));
      }
    } else if (entry.isFile() && entry.name.endsWith("_test.go")) {
      sources.push(absolute);
    }
  }
  return sources;
}

async function pythonTestSources(directory: string): Promise<string[]> {
  const sources: string[] = [];
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (!["node_modules", ".next", "dist"].includes(entry.name)) {
        sources.push(...(await pythonTestSources(absolute)));
      }
    } else if (entry.isFile() && entry.name.startsWith("test_") && entry.name.endsWith(".py")) {
      sources.push(absolute);
    }
  }
  return sources;
}

export async function validateEventContracts(): Promise<void> {
  const files = (await fs.readdir(eventsDirectory)).filter((file) => file.endsWith(".json")).sort();
  assertContract(files.length > 0, "contracts/events", "no event schemas found");

  const eventTypes = new Map<string, string>();
  const failures: string[] = [];
  for (const file of files) {
    try {
      assertContract(file.endsWith(".v1.json"), file, "event schema filename must be versioned");
      const schema = JSON.parse(await fs.readFile(path.join(eventsDirectory, file), "utf8")) as {
        $id?: string;
        $schema?: string;
        examples?: unknown[];
        required?: unknown;
        additionalProperties?: unknown;
        properties?: Record<string, any>;
      };
      const required = new Set(Array.isArray(schema.required) ? schema.required : []);
      const properties = schema.properties ?? {};
      const eventType = properties.type?.const;
      const expectedType = file.replace(/\.json$/, "");

      assertContract(
        schema.$id?.endsWith(`/events/${file}`),
        file,
        "$id must end with its contract path",
      );
      assertContract(schema.$schema, file, "$schema is required");
      assertContract(eventType === expectedType, file, `type.const must equal ${expectedType}`);
      for (const field of ["specversion", "type", "source", "id", "time", "tenant_id", "data"]) {
        assertContract(required.has(field), file, `CloudEvent field ${field} must be required`);
        assertContract(properties[field], file, `CloudEvent field ${field} must have a schema`);
      }
      assertContract(
        properties.specversion.const === "1.0",
        file,
        "specversion must be CloudEvents 1.0",
      );
      assertContract(properties.tenant_id.type === "string", file, "tenant_id must be a string");
      assertContract(properties.data.type === "object", file, "data must be an object");
      assertContract(
        schema.additionalProperties === false,
        file,
        "CloudEvent envelope must set additionalProperties to false",
      );
      assertContract(
        properties.data.additionalProperties === false,
        file,
        "CloudEvent data must set additionalProperties to false",
      );
      assertContract(
        !eventTypes.has(eventType),
        file,
        `duplicate event type also declared by ${eventTypes.get(eventType)}`,
      );
      eventTypes.set(eventType, file);

      const validator = String(schema.$schema).includes("2020-12") ? draft2020 : draft7;
      const validate = validator.compile(schema as AnySchema);
      assertContract(
        Array.isArray(schema.examples) && schema.examples.length > 0,
        file,
        "at least one example payload is required",
      );
      for (const [index, example] of schema.examples.entries()) {
        assertContract(
          validate(example),
          file,
          `example ${index + 1} does not satisfy its schema: ${validator.errorsText(validate.errors)}`,
        );
      }
    } catch (error) {
      failures.push(error instanceof Error ? error.message : `${file}: ${String(error)}`);
    }
  }

  const runtimeTypes = new Map<string, Set<string>>();
  const eventPattern = /["']([a-z][a-z0-9_-]*(?:\.[a-z0-9_-]+)+\.v1)["']/g;
  const contractTypeSet = new Set(eventTypes.keys());
  const producerServices = new Set<string>();
  const pythonProducerServices = new Set<string>();
  for (const source of await productionSources(appsDirectory)) {
    const contents = await fs.readFile(source, "utf8");
    const location = path.relative(root, source);
    if (contents.includes("tenancy.NewCloudEvent(")) {
      const [, service] = location.split(path.sep);
      if (service) producerServices.add(service);
    }
    if (source.endsWith(".py") && contents.includes("events.envelope import encode_event")) {
      const [, service] = location.split(path.sep);
      if (service) pythonProducerServices.add(service);
    }
    for (const match of contents.matchAll(eventPattern)) {
      const eventType = match[1];
      if (!eventType) continue;
      const locations = runtimeTypes.get(eventType) ?? new Set<string>();
      locations.add(location);
      runtimeTypes.set(eventType, locations);
    }
    failures.push(...runtimeEventFailures(contents, location, contractTypeSet));
  }
  const schemaBackedServices = new Set<string>();
  for (const source of await goTestSources(appsDirectory)) {
    const contents = await fs.readFile(source, "utf8");
    if (!contents.includes("AssertEventContract(")) continue;
    const location = path.relative(root, source);
    const [, service] = location.split(path.sep);
    if (service) schemaBackedServices.add(service);
  }
  failures.push(...producerCoverageFailures(producerServices, schemaBackedServices));
  const pythonSchemaBackedServices = new Set<string>();
  for (const source of await pythonTestSources(appsDirectory)) {
    const contents = await fs.readFile(source, "utf8");
    if (!contents.includes("assert_event_contract(")) continue;
    const location = path.relative(root, source);
    const [, service] = location.split(path.sep);
    if (service) pythonSchemaBackedServices.add(service);
  }
  failures.push(
    ...pythonProducerCoverageFailures(pythonProducerServices, pythonSchemaBackedServices),
  );
  for (const [eventType, locations] of [...runtimeTypes].sort(([left], [right]) =>
    left.localeCompare(right),
  )) {
    if (!eventTypes.has(eventType)) {
      failures.push(
        `missing ${eventType}.json for runtime event used by ${[...locations].join(", ")}`,
      );
    }
  }

  if (failures.length > 0) {
    throw new Error(
      `Event contract validation failed:\n${failures.map((failure) => `- ${failure}`).join("\n")}`,
    );
  }
  console.log(
    `Validated ${files.length} versioned AuraEDU CloudEvent schemas covering ${runtimeTypes.size} runtime event types.`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await validateEventContracts();
}
