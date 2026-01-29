import { getOutputFormat } from "../config.js";

// ANSI color codes
const colors = {
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  dim: "\x1b[2m",
  red: "\x1b[31m",
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  blue: "\x1b[34m",
  magenta: "\x1b[35m",
  cyan: "\x1b[36m",
};

export function color(text: string, ...codes: (keyof typeof colors)[]): string {
  if (!process.stdout.isTTY) return text;
  const prefix = codes.map((c) => colors[c]).join("");
  return `${prefix}${text}${colors.reset}`;
}

export function success(message: string): void {
  console.log(color("✓", "green"), message);
}

export function error(message: string): void {
  console.error(color("✗", "red"), message);
}

export function warn(message: string): void {
  console.warn(color("!", "yellow"), message);
}

export function info(message: string): void {
  console.log(color("→", "blue"), message);
}

export function outputJson(data: any): void {
  console.log(JSON.stringify(data, null, 2));
}

export function outputTable(
  headers: string[],
  rows: string[][],
  options: { compact?: boolean } = {}
): void {
  if (rows.length === 0) {
    console.log(color("(no results)", "dim"));
    return;
  }

  // Calculate column widths
  const widths = headers.map((h, i) =>
    Math.max(h.length, ...rows.map((r) => (r[i] || "").length))
  );

  // Print header
  if (!options.compact) {
    console.log(
      headers
        .map((h, i) => color(h.toUpperCase().padEnd(widths[i]), "bold"))
        .join("  ")
    );
    console.log(widths.map((w) => "-".repeat(w)).join("  "));
  }

  // Print rows
  for (const row of rows) {
    console.log(row.map((cell, i) => (cell || "").padEnd(widths[i])).join("  "));
  }
}

export function output(data: any, tableConfig?: { headers: string[]; rows: string[][] }): void {
  const format = getOutputFormat();
  if (format === "json") {
    outputJson(data);
  } else if (tableConfig) {
    outputTable(tableConfig.headers, tableConfig.rows);
  } else {
    outputJson(data);
  }
}

export function formatDate(date: string | Date): string {
  const d = typeof date === "string" ? new Date(date) : date;
  return d.toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export function formatDateTime(date: string | Date): string {
  const d = typeof date === "string" ? new Date(date) : date;
  return d.toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function truncate(str: string, maxLength: number): string {
  if (str.length <= maxLength) return str;
  return str.slice(0, maxLength - 3) + "...";
}
