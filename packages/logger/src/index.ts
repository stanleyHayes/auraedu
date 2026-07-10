export type LogLevel = "debug" | "info" | "warn" | "error";

interface LogEntry {
  timestamp: string;
  level: LogLevel;
  message: string;
  service?: string;
  [key: string]: unknown;
}

const SENSITIVE_KEYS = new Set([
  "password", "token", "accessToken", "refreshToken", "secret", "apiKey", "api_key",
  "authorization", "ssn", "phone", "email", "creditCard", "credit_card",
]);

const PII_PATTERNS = [
  { regex: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b/g, replacement: "[REDACTED_EMAIL]" },
  { regex: /\b\d{3}-\d{2}-\d{4}\b/g, replacement: "[REDACTED_SSN]" },
  { regex: /\b(?:\+\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b/g, replacement: "[REDACTED_PHONE]" },
];

function isSensitiveKey(key: string): boolean {
  const lower = key.toLowerCase();
  return SENSITIVE_KEYS.has(lower) || lower.endsWith("token") || lower.endsWith("secret");
}

export function redact(value: unknown): unknown {
  if (typeof value === "string") {
    return PII_PATTERNS.reduce((s, { regex, replacement }) => s.replace(regex, replacement), value);
  }
  if (Array.isArray(value)) return value.map(redact);
  if (value !== null && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [key, val] of Object.entries(value)) {
      out[key] = isSensitiveKey(key) ? "[REDACTED]" : redact(val);
    }
    return out;
  }
  return value;
}

function shouldLog(level: LogLevel, minLevel: LogLevel): boolean {
  const levels: LogLevel[] = ["debug", "info", "warn", "error"];
  return levels.indexOf(level) >= levels.indexOf(minLevel);
}

function getDefaultMinLevel(): LogLevel {
  if (typeof process !== "undefined" && process.env?.NEXT_PUBLIC_LOG_LEVEL) {
    const lvl = process.env.NEXT_PUBLIC_LOG_LEVEL as LogLevel;
    if (["debug", "info", "warn", "error"].includes(lvl)) return lvl;
  }
  return "info";
}

export interface LoggerOptions {
  service?: string;
  minLevel?: LogLevel;
}

export function createLogger(options: LoggerOptions = {}) {
  const service = options.service ?? "auraedu";
  const minLevel = options.minLevel ?? getDefaultMinLevel();

  function write(level: LogLevel, message: string, meta?: Record<string, unknown>) {
    if (!shouldLog(level, minLevel)) return;
    const entry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      message,
      service,
      ...(meta ? (redact(meta) as Record<string, unknown>) : undefined),
    };
    try {
      if (typeof window === "undefined" && typeof process !== "undefined") {
        console[level](JSON.stringify(entry));
        return;
      }
      console[level](entry);
    } catch {
      // ignore
    }
  }

  return {
    debug: (m: string, meta?: Record<string, unknown>) => write("debug", m, meta),
    info: (m: string, meta?: Record<string, unknown>) => write("info", m, meta),
    warn: (m: string, meta?: Record<string, unknown>) => write("warn", m, meta),
    error: (m: string, meta?: Record<string, unknown>) => write("error", m, meta),
  };
}

export const logger = createLogger({ service: "auraedu-web" });
