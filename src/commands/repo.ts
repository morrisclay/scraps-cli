import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, output, info } from "../utils/output.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    error(`Invalid format: "${ref}"`);
    info("Expected format: <store>/<repo>");
    info("Examples:");
    info("  scraps repo show alice/my-project");
    info("  scraps repo delete mystore/backend");
    process.exit(1);
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerRepoCommands(program: Command): void {
  const repo = program
    .command("repo")
    .description("Manage repositories")
    .addHelpText("after", `
Examples:
  scraps repo list                    # List repos in all your stores
  scraps repo list alice              # List repos in store "alice"
  scraps repo create alice/my-app     # Create repo "my-app" in store "alice"
  scraps repo show alice/my-app       # Show repo details
  scraps repo delete alice/my-app     # Delete a repo
`);

  repo
    .command("create <store/repo>")
    .description("Create a new repository")
    .addHelpText("after", `
Examples:
  scraps repo create alice/my-project
  scraps repo create myteam/backend-api
`)
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const result = await client.post(`/api/v1/stores/${store}/repos`, {
          name,
        });
        success(`Repository '${store}/${name}' created`);
        output(result.repo || result);
      } catch (e: any) {
        error(`Failed to create repo: ${e.message}`);
        process.exit(1);
      }
    });

  repo
    .command("list [store]")
    .description("List repositories (in a store, or all stores if omitted)")
    .addHelpText("after", `
Examples:
  scraps repo list              # List all repos across all your stores
  scraps repo list alice        # List repos only in store "alice"
`)
    .action(async (store) => {
      const client = requireAuth();
      try {
        if (store) {
          // List repos in specific store
          const result = await client.get(`/api/v1/stores/${store}/repos`);
          const repos = result.repos || result;
          if (repos.length === 0) {
            info(`No repositories in store "${store}"`);
            return;
          }
          output(repos, {
            headers: ["Name", "Default Branch", "Created"],
            rows: repos.map((r: any) => [
              r.name,
              r.default_branch || "main",
              new Date(r.created_at).toLocaleDateString(),
            ]),
          });
        } else {
          // List repos across all stores
          const storesResult = await client.get("/api/v1/stores");
          const stores = storesResult.stores || storesResult;

          let allRepos: any[] = [];
          for (const s of stores) {
            try {
              const result = await client.get(`/api/v1/stores/${s.slug}/repos`);
              const repos = result.repos || result;
              for (const r of repos) {
                allRepos.push({ ...r, store: s.slug });
              }
            } catch {
              // Skip stores we can't access
            }
          }

          if (allRepos.length === 0) {
            info("No repositories found");
            info("Create one with: scraps repo create <store>/<name>");
            return;
          }

          output(allRepos, {
            headers: ["Store", "Name", "Default Branch"],
            rows: allRepos.map((r: any) => [
              r.store,
              r.name,
              r.default_branch || "main",
            ]),
          });
        }
      } catch (e: any) {
        error(`Failed to list repos: ${e.message}`);
        process.exit(1);
      }
    });

  repo
    .command("show <store/repo>")
    .description("Show repository details")
    .addHelpText("after", `
Examples:
  scraps repo show alice/my-project
`)
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const result = await client.get(`/api/v1/stores/${store}/repos/${name}`);
        const r = result.repo || result;
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
    .command("delete <store/repo>")
    .description("Delete a repository")
    .addHelpText("after", `
Examples:
  scraps repo delete alice/old-project
`)
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
    .description("Manage repository collaborators")
    .addHelpText("after", `
Examples:
  scraps repo collaborators list alice/my-project
  scraps repo collaborators add alice/my-project bob --role write
  scraps repo collaborators remove alice/my-project bob
`);

  collaborators
    .command("list <store/repo>")
    .description("List repository collaborators")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const result = await client.get(
          `/api/v1/stores/${store}/repos/${name}/collaborators`
        );
        const collabs = result.collaborators || result;
        if (collabs.length === 0) {
          info("No collaborators");
          return;
        }
        output(collabs, {
          headers: ["Username", "Role", "Added"],
          rows: collabs.map((c: any) => [
            c.username,
            c.role,
            new Date(c.created_at).toLocaleDateString(),
          ]),
        });
      } catch (e: any) {
        error(`Failed to list collaborators: ${e.message}`);
        process.exit(1);
      }
    });

  collaborators
    .command("add <store/repo> <username>")
    .description("Add a collaborator to the repository")
    .option("-r, --role <role>", "Role: read, write, or admin", "read")
    .action(async (ref, username, opts) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        await client.post(`/api/v1/stores/${store}/repos/${name}/collaborators`, {
          username,
          role: opts.role,
        });
        success(`Added ${username} to ${store}/${name} with ${opts.role} access`);
      } catch (e: any) {
        error(`Failed to add collaborator: ${e.message}`);
        process.exit(1);
      }
    });

  collaborators
    .command("remove <store/repo> <username>")
    .description("Remove a collaborator from the repository")
    .action(async (ref, username) => {
      const client = requireAuth();
      const { store, repo: name } = parseRepoRef(ref);
      try {
        const result = await client.get(
          `/api/v1/stores/${store}/repos/${name}/collaborators`
        );
        const collabs = result.collaborators || result;
        const collab = collabs.find((c: any) => c.username === username);
        if (!collab) {
          error(`Collaborator '${username}' not found`);
          process.exit(1);
        }
        await client.delete(
          `/api/v1/stores/${store}/repos/${name}/collaborators/${collab.id}`
        );
        success(`Removed ${username} from ${store}/${name}`);
      } catch (e: any) {
        error(`Failed to remove collaborator: ${e.message}`);
        process.exit(1);
      }
    });
}
