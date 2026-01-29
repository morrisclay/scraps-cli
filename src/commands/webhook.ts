import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, output, truncate } from "../utils/output.js";

export function registerWebhookCommands(program: Command): void {
  const webhook = program.command("webhook").description("Manage webhooks");

  webhook
    .command("create <store>")
    .description("Create a webhook")
    .requiredOption("-u, --url <url>", "Webhook URL")
    .requiredOption("-e, --event <event>", "Event type (push, branch, etc)")
    .option("-r, --repo <repo>", "Limit to specific repository")
    .option("-s, --secret <secret>", "Webhook secret for verification")
    .action(async (store, opts) => {
      const client = requireAuth();

      const body: any = {
        url: opts.url,
        event: opts.event,
      };
      if (opts.repo) body.repo = opts.repo;
      if (opts.secret) body.secret = opts.secret;

      try {
        const result = await client.post(
          `/api/v1/stores/${store}/webhooks`,
          body
        );
        success("Webhook created");
        output(result);
      } catch (e: any) {
        error(`Failed to create webhook: ${e.message}`);
        process.exit(1);
      }
    });

  webhook
    .command("list <store>")
    .description("List webhooks")
    .action(async (store) => {
      const client = requireAuth();
      try {
        const webhooks = await client.get(`/api/v1/stores/${store}/webhooks`);
        output(webhooks, {
          headers: ["ID", "URL", "Event", "Repo", "Active"],
          rows: webhooks.map((w: any) => [
            w.id?.slice(0, 8) || "",
            truncate(w.url, 40),
            w.event,
            w.repo || "(all)",
            w.active !== false ? "yes" : "no",
          ]),
        });
      } catch (e: any) {
        error(`Failed to list webhooks: ${e.message}`);
        process.exit(1);
      }
    });

  webhook
    .command("update <store> <id>")
    .description("Update a webhook")
    .option("-u, --url <url>", "New webhook URL")
    .option("-e, --event <event>", "New event type")
    .option("--active <bool>", "Enable/disable webhook")
    .action(async (store, id, opts) => {
      const client = requireAuth();

      const body: any = {};
      if (opts.url) body.url = opts.url;
      if (opts.event) body.event = opts.event;
      if (opts.active !== undefined) {
        body.active = opts.active === "true";
      }

      try {
        await client.patch(`/api/v1/stores/${store}/webhooks/${id}`, body);
        success("Webhook updated");
      } catch (e: any) {
        error(`Failed to update webhook: ${e.message}`);
        process.exit(1);
      }
    });

  webhook
    .command("delete <store> <id>")
    .description("Delete a webhook")
    .action(async (store, id) => {
      const client = requireAuth();
      try {
        await client.delete(`/api/v1/stores/${store}/webhooks/${id}`);
        success("Webhook deleted");
      } catch (e: any) {
        error(`Failed to delete webhook: ${e.message}`);
        process.exit(1);
      }
    });
}
