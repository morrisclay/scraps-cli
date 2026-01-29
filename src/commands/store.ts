import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, output, outputTable, truncate } from "../utils/output.js";

export function registerStoreCommands(program: Command): void {
  const store = program.command("store").description("Manage stores");

  store
    .command("create <slug>")
    .description("Create a new store")
    .action(async (slug) => {
      const client = requireAuth();
      try {
        const result = await client.post("/api/v1/stores", { slug });
        success(`Store '${slug}' created`);
        output(result);
      } catch (e: any) {
        error(`Failed to create store: ${e.message}`);
        process.exit(1);
      }
    });

  store
    .command("list")
    .description("List stores you're a member of")
    .action(async () => {
      const client = requireAuth();
      try {
        const stores = await client.get("/api/v1/stores");
        output(stores, {
          headers: ["Slug", "Role", "Created"],
          rows: stores.map((s: any) => [
            s.slug,
            s.role || "owner",
            new Date(s.created_at).toLocaleDateString(),
          ]),
        });
      } catch (e: any) {
        error(`Failed to list stores: ${e.message}`);
        process.exit(1);
      }
    });

  store
    .command("show <slug>")
    .description("Show store details")
    .action(async (slug) => {
      const client = requireAuth();
      try {
        const s = await client.get(`/api/v1/stores/${slug}`);
        output(s, {
          headers: ["Field", "Value"],
          rows: [
            ["ID", s.id],
            ["Slug", s.slug],
            ["Owner", s.owner_id],
            ["Created", new Date(s.created_at).toLocaleDateString()],
          ],
        });
      } catch (e: any) {
        error(`Failed to get store: ${e.message}`);
        process.exit(1);
      }
    });

  store
    .command("delete <slug>")
    .description("Delete a store")
    .action(async (slug) => {
      const client = requireAuth();
      try {
        await client.delete(`/api/v1/stores/${slug}`);
        success(`Store '${slug}' deleted`);
      } catch (e: any) {
        error(`Failed to delete store: ${e.message}`);
        process.exit(1);
      }
    });

  // Members subcommand
  const members = store.command("members").description("Manage store members");

  members
    .command("list <slug>")
    .description("List store members")
    .action(async (slug) => {
      const client = requireAuth();
      try {
        const result = await client.get(`/api/v1/stores/${slug}/members`);
        output(result, {
          headers: ["Username", "Role", "Joined"],
          rows: result.map((m: any) => [
            m.username,
            m.role,
            new Date(m.created_at).toLocaleDateString(),
          ]),
        });
      } catch (e: any) {
        error(`Failed to list members: ${e.message}`);
        process.exit(1);
      }
    });

  members
    .command("add <slug> <username>")
    .description("Add a member to the store")
    .option("-r, --role <role>", "Role (admin, member, read)", "member")
    .action(async (slug, username, opts) => {
      const client = requireAuth();
      try {
        await client.post(`/api/v1/stores/${slug}/members`, {
          username,
          role: opts.role,
        });
        success(`Added ${username} to ${slug} as ${opts.role}`);
      } catch (e: any) {
        error(`Failed to add member: ${e.message}`);
        process.exit(1);
      }
    });

  members
    .command("update <slug> <username>")
    .description("Update member role")
    .requiredOption("-r, --role <role>", "New role (admin, member, read)")
    .action(async (slug, username, opts) => {
      const client = requireAuth();
      try {
        await client.patch(`/api/v1/stores/${slug}/members/${username}`, {
          role: opts.role,
        });
        success(`Updated ${username}'s role to ${opts.role}`);
      } catch (e: any) {
        error(`Failed to update member: ${e.message}`);
        process.exit(1);
      }
    });

  members
    .command("remove <slug> <username>")
    .description("Remove a member from the store")
    .action(async (slug, username) => {
      const client = requireAuth();
      try {
        await client.delete(`/api/v1/stores/${slug}/members/${username}`);
        success(`Removed ${username} from ${slug}`);
      } catch (e: any) {
        error(`Failed to remove member: ${e.message}`);
        process.exit(1);
      }
    });
}
