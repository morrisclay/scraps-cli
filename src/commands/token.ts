import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, info, output, truncate } from "../utils/output.js";

export function registerTokenCommands(program: Command): void {
  const token = program.command("token").description("Manage API keys and tokens");

  token
    .command("create")
    .description("Create a new API key or scoped token")
    .option("-n, --name <name>", "Token name")
    .option("--scoped", "Create a scoped token instead of API key")
    .option("-s, --store <store>", "Store slug (for scoped token)")
    .option("-r, --repo <repo>", "Repository name (for scoped token)")
    .option("-p, --permission <permission>", "Permission level (for scoped token)", "read")
    .option("--expires <days>", "Expiration in days (for scoped token)")
    .action(async (opts) => {
      const client = requireAuth();

      if (opts.scoped) {
        // Create scoped token
        const body: any = {};
        if (opts.name) body.name = opts.name;
        if (opts.store) body.store = opts.store;
        if (opts.repo) body.repo = opts.repo;
        if (opts.permission) body.permission = opts.permission;
        if (opts.expires) {
          const expiresAt = new Date();
          expiresAt.setDate(expiresAt.getDate() + parseInt(opts.expires));
          body.expires_at = expiresAt.toISOString();
        }

        try {
          const result = await client.post("/api/v1/tokens", body);
          success("Scoped token created");
          info(`Token: ${result.token}`);
          if (result.scope) {
            info(`Scope: ${JSON.stringify(result.scope)}`);
          }
        } catch (e: any) {
          error(`Failed to create token: ${e.message}`);
          process.exit(1);
        }
      } else {
        // Create API key
        const body: any = {};
        if (opts.name) body.name = opts.name;

        try {
          const result = await client.post("/api/v1/api-keys", body);
          success("API key created");
          info(`Key: ${result.api_key || result.key}`);
          info("Save this key - it won't be shown again");
        } catch (e: any) {
          error(`Failed to create API key: ${e.message}`);
          process.exit(1);
        }
      }
    });

  token
    .command("list")
    .description("List API keys and tokens")
    .option("--keys", "List only API keys")
    .option("--tokens", "List only scoped tokens")
    .action(async (opts) => {
      const client = requireAuth();

      if (!opts.tokens) {
        // List API keys
        try {
          const keys = await client.get("/api/v1/api-keys");
          console.log("\nAPI Keys:");
          output(keys, {
            headers: ["ID", "Name", "Created", "Last Used"],
            rows: keys.map((k: any) => [
              k.id?.slice(0, 8) || "",
              k.name || "(unnamed)",
              new Date(k.created_at).toLocaleDateString(),
              k.last_used_at
                ? new Date(k.last_used_at).toLocaleDateString()
                : "never",
            ]),
          });
        } catch (e: any) {
          error(`Failed to list API keys: ${e.message}`);
        }
      }

      if (!opts.keys) {
        // List scoped tokens
        try {
          const tokens = await client.get("/api/v1/tokens");
          console.log("\nScoped Tokens:");
          output(tokens, {
            headers: ["ID", "Name", "Scope", "Expires"],
            rows: tokens.map((t: any) => [
              t.id?.slice(0, 8) || "",
              t.name || "(unnamed)",
              truncate(JSON.stringify(t.scope || {}), 30),
              t.expires_at
                ? new Date(t.expires_at).toLocaleDateString()
                : "never",
            ]),
          });
        } catch (e: any) {
          error(`Failed to list tokens: ${e.message}`);
        }
      }
    });

  token
    .command("revoke <id>")
    .description("Revoke an API key or token")
    .option("--token", "Revoke a scoped token (default is API key)")
    .action(async (id, opts) => {
      const client = requireAuth();
      const endpoint = opts.token ? "/api/v1/tokens" : "/api/v1/api-keys";

      try {
        await client.delete(`${endpoint}/${id}`);
        success(`${opts.token ? "Token" : "API key"} revoked`);
      } catch (e: any) {
        error(`Failed to revoke: ${e.message}`);
        process.exit(1);
      }
    });
}
