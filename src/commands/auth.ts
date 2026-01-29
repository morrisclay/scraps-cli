import { Command } from "commander";
import { createInterface } from "readline";
import {
  getCredential,
  loadConfig,
  removeCredential,
  setCredential,
} from "../config.js";
import { ScrapsClient } from "../api.js";
import { success, error, info, output } from "../utils/output.js";

function prompt(question: string): Promise<string> {
  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
  });
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      rl.close();
      resolve(answer);
    });
  });
}

export function registerAuthCommands(program: Command): void {
  program
    .command("login")
    .description("Login with API key")
    .option("-k, --key <key>", "API key (prompts if not provided)")
    .option("-h, --host <host>", "Server host")
    .action(async (opts) => {
      const config = loadConfig();
      const host = opts.host || config.default_host;

      let apiKey = opts.key;
      if (!apiKey) {
        apiKey = await prompt("API key: ");
      }

      if (!apiKey) {
        error("API key is required");
        process.exit(1);
      }

      // Validate the API key by fetching user info
      const client = new ScrapsClient(host, apiKey);
      try {
        const user = await client.get("/api/v1/me");
        setCredential(host, {
          api_key: apiKey,
          user_id: user.id,
          username: user.username,
        });
        success(`Logged in as ${user.username} on ${host}`);
      } catch (e: any) {
        error(`Login failed: ${e.message}`);
        process.exit(1);
      }
    });

  program
    .command("logout")
    .description("Clear saved credentials")
    .option("-h, --host <host>", "Server host")
    .action((opts) => {
      const config = loadConfig();
      const host = opts.host || config.default_host;
      removeCredential(host);
      success(`Logged out from ${host}`);
    });

  program
    .command("whoami")
    .description("Show current user info")
    .option("-h, --host <host>", "Server host")
    .action(async (opts) => {
      const config = loadConfig();
      const host = opts.host || config.default_host;
      const cred = getCredential(host);

      if (!cred) {
        error(`Not logged in to ${host}`);
        process.exit(1);
      }

      const client = new ScrapsClient(host, cred.api_key);
      try {
        const user = await client.get("/api/v1/me");
        output(user, {
          headers: ["Field", "Value"],
          rows: [
            ["Username", user.username],
            ["Email", user.email],
            ["ID", user.id],
            ["Host", host],
          ],
        });
      } catch (e: any) {
        error(`Failed to fetch user info: ${e.message}`);
        process.exit(1);
      }
    });

  program
    .command("signup")
    .description("Create a new account")
    .option("-u, --username <username>", "Username")
    .option("-e, --email <email>", "Email")
    .option("-h, --host <host>", "Server host")
    .action(async (opts) => {
      const config = loadConfig();
      const host = opts.host || config.default_host;

      const username = opts.username || (await prompt("Username: "));
      const email = opts.email || (await prompt("Email: "));

      if (!username || !email) {
        error("Username and email are required");
        process.exit(1);
      }

      const client = new ScrapsClient(host);
      try {
        const result = await client.post("/api/v1/signup", { username, email });

        // Auto-login with the returned API key
        setCredential(host, {
          api_key: result.api_key,
          user_id: result.user.id,
          username: result.user.username,
        });

        success(`Account created for ${username}`);
        info(`API key: ${result.api_key}`);
        info("You are now logged in");
      } catch (e: any) {
        error(`Signup failed: ${e.message}`);
        process.exit(1);
      }
    });
}
