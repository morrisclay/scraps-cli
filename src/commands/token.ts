import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, info, output, truncate } from "../utils/output.js";

export function registerTokenCommands(program: Command): void {
  const token = program
    .command("token")
    .description("Manage API keys and scoped tokens")
    .addHelpText("after", `
API Keys: Full access tokens for your account
Scoped Tokens: Limited access tokens for specific stores/repos (great for agents)

Examples:
  scraps token create --name "CI bot"                    # Create API key
  scraps token create --scoped -s mystore -p read        # Create scoped token
  scraps token list                                      # List all tokens
  scraps token revoke abc12345                           # Revoke an API key
  scraps token revoke abc12345 --token                   # Revoke a scoped token
`);

  token
    .command("create")
    .description("Create a new API key or scoped token")
    .option("-n, --name <name>", "Token name")
    .option("--scoped", "Create a scoped token instead of API key")
    .option("-s, --store <store>", "Store slug (for scoped token)")
    .option("-r, --repo <repo>", "Repository name (for scoped token)")
    .option("-p, --permission <permission>", "Permission level (for scoped token)", "read")
    .option("--expires <days>", "Expiration in days (for scoped token)")
    .addHelpText("after", `
Examples:
  # Create a named API key (full access)
  scraps token create --name "My laptop"

  # Create a scoped token for read access to a store
  scraps token create --scoped -s alice -p read

  # Create a scoped token for write access to a specific repo
  scraps token create --scoped -s alice -r my-project -p write --name "CI deploy"

  # Create an expiring token (30 days)
  scraps token create --scoped -s mystore -p read --expires 30

Permission levels: read, write, admin
`)
    .action(async (opts) => {
      const client = requireAuth();

      if (opts.scoped) {
        // Create scoped token - need to resolve store slug to store_id first
        try {
          // Build scope object
          const scope: any = {};

          if (opts.store) {
            // Resolve store slug to store_id
            const storeResult = await client.get(`/api/v1/stores/${opts.store}`);
            const store = storeResult.store || storeResult;
            scope.store_id = store.id;
          }

          if (opts.repo) {
            scope.repos = [opts.repo];
          }

          if (opts.permission) {
            scope.permissions = [opts.permission];
          }

          const body: any = { scope };
          if (opts.name) body.label = opts.name;
          if (opts.expires) {
            const expiresAt = new Date();
            expiresAt.setDate(expiresAt.getDate() + parseInt(opts.expires));
            body.expires_at = expiresAt.toISOString();
          }

          const result = await client.post("/api/v1/scoped-tokens", body);
          success("Scoped token created");
          info(`Token: ${result.raw_key}`);
          info("Save this token - it won't be shown again");
          if (result.scoped_token?.scope) {
            info(`Scope: ${JSON.stringify(result.scoped_token.scope)}`);
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
          info(`Key: ${result.raw_key}`);
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
    .addHelpText("after", `
Examples:
  scraps token list              # List all API keys and scoped tokens
  scraps token list --keys       # List only API keys
  scraps token list --tokens     # List only scoped tokens
`)
    .action(async (opts) => {
      const client = requireAuth();

      if (!opts.tokens) {
        // List API keys
        try {
          const result = await client.get("/api/v1/api-keys");
          const keys = result.api_keys || result;
          console.log("\nAPI Keys:");
          if (!Array.isArray(keys) || keys.length === 0) {
            info("No API keys found");
          } else {
            output(keys, {
              headers: ["ID", "Label", "Prefix", "Created", "Last Used"],
              rows: keys.map((k: any) => [
                k.id?.slice(0, 8) || "",
                k.label || "(unnamed)",
                k.key_prefix || "",
                new Date(k.created_at).toLocaleDateString(),
                k.last_used_at
                  ? new Date(k.last_used_at).toLocaleDateString()
                  : "never",
              ]),
            });
          }
        } catch (e: any) {
          error(`Failed to list API keys: ${e.message}`);
        }
      }

      if (!opts.keys) {
        // List scoped tokens
        try {
          const result = await client.get("/api/v1/scoped-tokens");
          const tokens = result.scoped_tokens || result;
          console.log("\nScoped Tokens:");
          if (!Array.isArray(tokens) || tokens.length === 0) {
            info("No scoped tokens found");
          } else {
            output(tokens, {
              headers: ["ID", "Label", "Scope", "Expires"],
              rows: tokens.map((t: any) => [
                t.id?.slice(0, 8) || "",
                t.label || "(unnamed)",
                truncate(JSON.stringify(t.scope || {}), 30),
                t.expires_at
                  ? new Date(t.expires_at).toLocaleDateString()
                  : "never",
              ]),
            });
          }
        } catch (e: any) {
          error(`Failed to list tokens: ${e.message}`);
        }
      }
    });

  token
    .command("revoke <id>")
    .description("Revoke an API key or token")
    .option("--token", "Revoke a scoped token (default is API key)")
    .addHelpText("after", `
Examples:
  scraps token revoke abc12345              # Revoke an API key
  scraps token revoke abc12345 --token      # Revoke a scoped token

Use 'scraps token list' to find token IDs.
`)
    .action(async (id, opts) => {
      const client = requireAuth();
      const endpoint = opts.token ? "/api/v1/scoped-tokens" : "/api/v1/api-keys";

      try {
        await client.delete(`${endpoint}/${id}`);
        success(`${opts.token ? "Token" : "API key"} revoked`);
      } catch (e: any) {
        error(`Failed to revoke: ${e.message}`);
        process.exit(1);
      }
    });
}
