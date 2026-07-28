/**
 * Client-safe CSV helpers for the student bulk-import dialog. Server actions live in
 * admin-import-actions.ts — a "use server" module may only export async functions, so
 * the shared constants and parsers stay here.
 */

/** Exact header row the student service parses (apps/student-service import handler). */
export const STUDENT_IMPORT_HEADERS = [
  "first_name",
  "last_name",
  "date_of_birth",
  "gender",
  "relationship",
  "guardian_first_name",
  "guardian_last_name",
  "guardian_phone",
  "guardian_email",
  "user_id",
  "guardian_user_id",
] as const;

const EXAMPLE_ROW = [
  "Ama",
  "Mensah",
  "2012-04-18",
  "female",
  "mother",
  "Efua",
  "Mensah",
  "+233201234567",
  "efua.mensah@example.com",
  "",
  "",
];

/** CSV template offered for download in the import dialog. */
export function studentImportTemplateCsv(): string {
  return `${STUDENT_IMPORT_HEADERS.join(",")}\n${EXAMPLE_ROW.join(",")}\n`;
}

/** Minimal CSV line splitter — quoted cells with commas are joined, not split. */
export function parseCsvLine(line: string): string[] {
  const cells: string[] = [];
  let current = "";
  let quoted = false;
  for (let index = 0; index < line.length; index += 1) {
    const char = line[index];
    if (quoted) {
      if (char === '"' && line[index + 1] === '"') {
        current += '"';
        index += 1;
      } else if (char === '"') {
        quoted = false;
      } else {
        current += char;
      }
    } else if (char === '"') {
      quoted = true;
    } else if (char === ",") {
      cells.push(current);
      current = "";
    } else {
      current += char;
    }
  }
  cells.push(current);
  return cells.map((cell) => cell.trim());
}

export interface CsvPreview {
  headers: string[];
  rows: string[][];
  rowCount: number;
}

/** Preview the header and first rows of CSV text; returns null when no header row exists. */
export function previewCsv(text: string, maxRows = 5): CsvPreview | null {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
  const [headerLine, ...dataLines] = lines;
  if (!headerLine) return null;
  const headers = parseCsvLine(headerLine).map((header) => header.toLowerCase());
  return {
    headers,
    rows: dataLines.slice(0, maxRows).map(parseCsvLine),
    rowCount: dataLines.length,
  };
}
