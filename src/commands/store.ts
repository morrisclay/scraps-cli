import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, output, outputTable, truncate } from "../utils/output.js";

export function registerStoreCommands(program: Command): void {
  const store = program
    .command("store")
    .description("Manage stores (containers for repositories)")
    .addHelpText("after", `
Stores are containers for repositories. Each store has a unique slug
used in URLs and can have multiple members with different roles.

Examples:
  scraps store create myteam                 # Create a store
  scraps store list                          # List your stores
  scraps store show myteam                   # Show store details
  scraps store members list myteam           # List store members
  scraps store members add myteam bob        # Add a member
`);

  store
    .command("create <slug>")
    .description("Create a new store")
    .addHelpText("after", `
The slug is a unique identifier used in URLs. Use lowercase letters,
numbers, and hyphens.

Examples:
  scraps store create alice
  scraps store create my-team
  scraps store create project-x
`)
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
    .addHelpText("after", `
Shows all stores where you are an owner, admin, or member.

Example:
  scraps store list
`)
    .action(async () => {
      const client = requireAuth();
      try {
        const result = await client.get("/api/v1/stores");
        const stores = result.stores || result;
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
    .addHelpText("after", `
Example:
  scraps store show myteam
`)
    .action(async (slug) => {
      const client = requireAuth();
      try {
        const result = await client.get(`/api/v1/stores/${slug}`);
        const s = result.store || result;
        output(s, {
          headers: ["Field", "Value"],
          rows: [
            ["ID", s.id],
            ["Slug", s.slug],
            ["Owner", s.owner_user_id || s.owner_id],
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
    .description("Delete a store (and all its repositories)")
    .addHelpText("after", `
WARNING: This permanently deletes the store and all repositories in it.

Example:
  scraps store delete old-project
`)
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
  const members = store
    .command("members")
    .description("Manage store members")
    .addHelpText("after", `
Roles: owner, admin, member, read

Examples:
  scraps store members list myteam
  scraps store members add myteam bob --role admin
  scraps store members update myteam bob --role member
  scraps store members remove myteam bob
`);

  members
    .command("list <slug>")
    .description("List store members")
    .addHelpText("after", `
Example:
  scraps store members list myteam
`)
    .action(async (slug) => {
      const client = requireAuth();
      try {
        const result = await client.get(`/api/v1/stores/${slug}/members`);
        const members = result.members || result;
        output(members, {
          headers: ["Username", "Role", "Joined"],
          rows: members.map((m: any) => [
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
    .addHelpText("after", `
Roles:
  admin   - Can manage repos, members, and settings
  member  - Can create and manage repos
  read    - Can only read repositories

Examples:
  scraps store members add myteam bob                  # Add as member (default)
  scraps store members add myteam alice --role admin   # Add as admin
  scraps store members add myteam viewer --role read   # Add as read-only
`)
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
    .addHelpText("after", `
Example:
  scraps store members update myteam bob --role admin
`)
    .action(async (slug, username, opts) => {
      const client = requireAuth();
      try {
        // Look up userId from username
        const result = await client.get(`/api/v1/stores/${slug}/members`);
        const members = result.members || result;
        const member = members.find((m: any) => m.username === username);
        if (!member) {
          error(`Member '${username}' not found in store`);
          process.exit(1);
        }
        await client.patch(`/api/v1/stores/${slug}/members/${member.id}`, {
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
    .addHelpText("after", `
Example:
  scraps store members remove myteam bob
`)
    .action(async (slug, username) => {
      const client = requireAuth();
      try {
        // Look up userId from username
        const result = await client.get(`/api/v1/stores/${slug}/members`);
        const members = result.members || result;
        const member = members.find((m: any) => m.username === username);
        if (!member) {
          error(`Member '${username}' not found in store`);
          process.exit(1);
        }
        await client.delete(`/api/v1/stores/${slug}/members/${member.id}`);
        success(`Removed ${username} from ${slug}`);
      } catch (e: any) {
        error(`Failed to remove member: ${e.message}`);
        process.exit(1);
      }
    });
}
