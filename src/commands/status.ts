import { Command } from "commander";
import { ScrapsClient } from "../api.js";
import { loadConfig, getCredential } from "../config.js";
import { info, color, error } from "../utils/output.js";

export function registerStatusCommand(program: Command): void {
  program
    .command("status")
    .description("Show current status (user, host, config)")
    .addHelpText("after", `
Shows your current login status, configured host, and account info.

Example:
  scraps status
`)
    .action(async () => {
      const config = loadConfig();
      const host = config.default_host;
      const cred = getCredential(host);

      console.log(color("Scraps CLI Status", "bold"));
      console.log();

      // Host
      console.log(`${color("Host:", "cyan")}     ${host}`);

      // Auth status
      if (!cred) {
        console.log(`${color("Auth:", "cyan")}     ${color("Not logged in", "yellow")}`);
        info("Run 'scraps login' to authenticate");
        return;
      }

      console.log(`${color("Auth:", "cyan")}     ${color("Logged in", "green")}`);
      console.log(`${color("User:", "cyan")}     ${cred.username}`);

      // Verify token is still valid
      const client = new ScrapsClient(host, cred.api_key);
      try {
        const result = await client.get("/api/v1/user");
        const user = result.user || result;
        console.log(`${color("Email:", "cyan")}    ${user.email}`);
        console.log(`${color("User ID:", "cyan")}  ${user.id.slice(0, 8)}...`);
      } catch (e: any) {
        console.log(`${color("Warning:", "yellow")} Could not verify token: ${e.message}`);
      }

      // Config
      console.log();
      console.log(`${color("Output:", "cyan")}   ${config.output_format}`);

      // Show stores count
      try {
        const storesResult = await client.get("/api/v1/stores");
        const stores = storesResult.stores || storesResult;
        console.log(`${color("Stores:", "cyan")}   ${stores.length} accessible`);
      } catch {
        // Ignore
      }
    });
}
