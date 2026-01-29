import { existsSync, mkdirSync, readFileSync, writeFileSync } from "fs";
import { homedir } from "os";
import { join } from "path";

const CONFIG_DIR = join(homedir(), ".scraps");
const CONFIG_FILE = join(CONFIG_DIR, "config.json");
const CREDENTIALS_FILE = join(CONFIG_DIR, "credentials.json");

export interface Config {
  default_host: string;
  output_format: "table" | "json";
}

export interface Credential {
  api_key: string;
  user_id: string;
  username: string;
}

export type Credentials = Record<string, Credential>;

function ensureConfigDir(): void {
  if (!existsSync(CONFIG_DIR)) {
    mkdirSync(CONFIG_DIR, { recursive: true, mode: 0o700 });
  }
}

export function loadConfig(): Config {
  ensureConfigDir();
  if (!existsSync(CONFIG_FILE)) {
    return {
      default_host: "https://api.scraps.sh",
      output_format: "table",
    };
  }
  return JSON.parse(readFileSync(CONFIG_FILE, "utf-8"));
}

export function saveConfig(config: Config): void {
  ensureConfigDir();
  writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2), { mode: 0o600 });
}

export function loadCredentials(): Credentials {
  ensureConfigDir();
  if (!existsSync(CREDENTIALS_FILE)) {
    return {};
  }
  return JSON.parse(readFileSync(CREDENTIALS_FILE, "utf-8"));
}

export function saveCredentials(creds: Credentials): void {
  ensureConfigDir();
  writeFileSync(CREDENTIALS_FILE, JSON.stringify(creds, null, 2), {
    mode: 0o600,
  });
}

export function getCredential(host?: string): Credential | null {
  const config = loadConfig();
  const creds = loadCredentials();
  const targetHost = host || config.default_host;
  return creds[targetHost] || null;
}

export function setCredential(
  host: string,
  credential: Credential
): void {
  const creds = loadCredentials();
  creds[host] = credential;
  saveCredentials(creds);
}

export function removeCredential(host?: string): void {
  const config = loadConfig();
  const creds = loadCredentials();
  const targetHost = host || config.default_host;
  delete creds[targetHost];
  saveCredentials(creds);
}

export function getHost(): string {
  return loadConfig().default_host;
}

export function getOutputFormat(): "table" | "json" {
  return loadConfig().output_format;
}
