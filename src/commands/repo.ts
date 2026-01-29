import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, output } from "../utils/output.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    throw new Error("Invalid repo reference. Use format: store/repo");
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerRepoCommands(program: Command): void {
  const repo = program.command("repo").description("Manage repositories");

  repo
    .command("create <store/name>")
    .description("Create a new repository")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const result = await client.post(`/api/v1/stores/${store}/repos`, {
          name,
        });
        success(`Repository '${store}/${name}' created`);
        output(result);
      } catch (e: any) {
        error(`Failed to create repo: ${e.message}`);
        process.exit(1);
      }
    });

  repo
    .command("list <store>")
    .description("List repositories in a store")
    .action(async (store) => {
      const client = requireAuth();
      try {
        const repos = await client.get(`/api/v1/stores/${store}/repos`);
        output(repos, {
          headers: ["Name", "Default Branch", "Created"],
          rows: repos.map((r: any) => [
            r.name,
            r.default_branch || "main",
            new Date(r.created_at).toLocaleDateString(),
          ]),
        });
      } catch (e: any) {
        error(`Failed to list repos: ${e.message}`);
        process.exit(1);
      }
    });

  repo
    .command("show <store/name>")
    .description("Show repository details")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const r = await client.get(`/api/v1/stores/${store}/repos/${name}`);
        output(r, {
          headers: ["Field", "Value"],
          rows: [
            ["ID", r.id],
            ["Name", r.name],
            ["Store", store],
            ["Default Branch", r.default_branch || "main"],
            ["Created", new Date(r.created_at).toLocaleDateString()],
          ],
        });
      } catch (e: any) {
        error(`Failed to get repo: ${e.message}`);
        process.exit(1);
      }
    });

  repo
    .command("delete <store/name>")
    .description("Delete a repository")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        await client.delete(`/api/v1/stores/${store}/repos/${name}`);
        success(`Repository '${store}/${name}' deleted`);
      } catch (e: any) {
        error(`Failed to delete repo: ${e.message}`);
        process.exit(1);
      }
    });

  // Collaborators subcommand
  const collaborators = repo
    .command("collaborators")
    .description("Manage repository collaborators");

  collaborators
    .command("list <store/name>")
    .description("List repository collaborators")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const result = await client.get(
          `/api/v1/stores/${store}/repos/${name}/collaborators`
        );
        output(result, {
          headers: ["Username", "Permission", "Added"],
          rows: result.map((c: any) => [
            c.username,
            c.permission,
            new Date(c.created_at).toLocaleDateString(),
          ]),
        });
      } catch (e: any) {
        error(`Failed to list collaborators: ${e.message}`);
        process.exit(1);
      }
    });

  collaborators
    .command("add <store/name> <username>")
    .description("Add a collaborator to the repository")
    .option("-p, --permission <permission>", "Permission (write, read)", "read")
    .action(async (ref, username, opts) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        await client.post(`/api/v1/stores/${store}/repos/${name}/collaborators`, {
          username,
          permission: opts.permission,
        });
        success(`Added ${username} to ${store}/${name} with ${opts.permission} access`);
      } catch (e: any) {
        error(`Failed to add collaborator: ${e.message}`);
        process.exit(1);
      }
    });

  collaborators
    .command("remove <store/name> <username>")
    .description("Remove a collaborator from the repository")
    .action(async (ref, username) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        await client.delete(
          `/api/v1/stores/${store}/repos/${name}/collaborators/${username}`
        );
        success(`Removed ${username} from ${store}/${name}`);
      } catch (e: any) {
        error(`Failed to remove collaborator: ${e.message}`);
        process.exit(1);
      }
    });
}
