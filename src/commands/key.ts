import { Command } from "commander";
import { loadConfig, setCredential } from "../config.js";
import { ScrapsClient } from "../api.js";
import { success, error, info } from "../utils/output.js";

export function registerKeyCommands(program: Command): void {
  const key = program.command("key").description("API key management");

  key
    .command("reset-request <email>")
    .description("Request an API key reset email")
    .option("-h, --host <host>", "Server host")
    .addHelpText("after", `
Sends a reset link to your email address. Use the token from
the email with 'scraps key reset-confirm' to get a new API key.

Example:
  scraps key reset-request user@example.com
`)
    .action(async (email: string, opts) => {
      const config = loadConfig();
      const host = opts.host || config.default_host;
      const client = new ScrapsClient(host);

      try {
        await client.post("/api/v1/reset-api-key", { email });
        success(`Reset email sent to ${email}`);
        info(`Check your email and run: scraps key reset-confirm <token>`);
      } catch (e: any) {
        error(`Failed to request reset: ${e.message}`);
        process.exit(1);
      }
    });

  key
    .command("reset-confirm <token>")
    .description("Confirm API key reset with token from email")
    .option("-h, --host <host>", "Server host")
    .option("--no-login", "Don't save credential after reset")
    .addHelpText("after", `
Confirms your API key reset using the token from your email.
Your old API keys will be revoked and a new one will be returned.

Examples:
  scraps key reset-confirm abc123           # Reset and auto-login
  scraps key reset-confirm abc123 --no-login  # Just show the key
`)
    .action(async (token: string, opts) => {
      const config = loadConfig();
      const host = opts.host || config.default_host;
      const client = new ScrapsClient(host);

      try {
        const result = await client.get(
          `/api/v1/confirm-reset?token=${encodeURIComponent(token)}`
        );

        success("API key reset successful");
        info(`API key: ${result.api_key}`);

        if (opts.login !== false) {
          setCredential(host, {
            api_key: result.api_key,
            user_id: result.user_id,
            username: result.username,
          });
          info(`You are now logged in as ${result.username}`);
        }
      } catch (e: any) {
        error(`Failed to confirm reset: ${e.message}`);
        process.exit(1);
      }
    });
}
