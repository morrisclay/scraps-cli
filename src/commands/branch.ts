import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, output, info } from "../utils/output.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    error(`Invalid format: "${ref}"`);
    info("Expected format: <store>/<repo>");
    info("Examples:");
    info("  scraps branch list alice/my-project");
    info("  scraps branch create alice/my-project feature-x");
    process.exit(1);
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerBranchCommands(program: Command): void {
  const branch = program
    .command("branch")
    .description("Manage branches")
    .addHelpText("after", `
Examples:
  scraps branch list alice/my-project                    # List all branches
  scraps branch create alice/my-project feature-x        # Create branch from default
  scraps branch create alice/my-project hotfix -f main   # Create from specific branch
  scraps branch delete alice/my-project old-branch       # Delete a branch
`);

  branch
    .command("list <store/repo>")
    .description("List branches in a repository")
    .addHelpText("after", `
Examples:
  scraps branch list alice/my-project
  scraps branch list myteam/backend
`)
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      try {
        const result = await client.get(
          `/api/v1/stores/${store}/repos/${repo}/branches`
        );
        const branches = result.branches || result;
        if (branches.length === 0) {
          info("No branches (empty repository)");
          return;
        }
        output(branches, {
          headers: ["Name", "SHA"],
          rows: branches.map((b: any) => [
            b.name,
            b.sha?.slice(0, 7) || b.commit?.slice(0, 7) || "",
          ]),
        });
      } catch (e: any) {
        error(`Failed to list branches: ${e.message}`);
        process.exit(1);
      }
    });

  branch
    .command("create <store/repo> <name>")
    .description("Create a new branch")
    .option("-f, --from <branch>", "Source branch to create from", "main")
    .addHelpText("after", `
Examples:
  scraps branch create alice/my-project feature-login
  scraps branch create alice/my-project hotfix --from production
`)
    .action(async (ref, name, opts) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      try {
        await client.post(`/api/v1/stores/${store}/repos/${repo}/branches`, {
          name,
          source: opts.from,
        });
        success(`Branch '${name}' created from '${opts.from}'`);
      } catch (e: any) {
        error(`Failed to create branch: ${e.message}`);
        if (e.message.includes("not found")) {
          info(`Hint: Make sure branch '${opts.from}' exists. Use --from to specify a different source.`);
        }
        process.exit(1);
      }
    });

  branch
    .command("delete <store/repo> <name>")
    .description("Delete a branch")
    .addHelpText("after", `
Examples:
  scraps branch delete alice/my-project old-feature
`)
    .action(async (ref, name) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      try {
        await client.delete(
          `/api/v1/stores/${store}/repos/${repo}/branches/${encodeURIComponent(name)}`
        );
        success(`Branch '${name}' deleted`);
      } catch (e: any) {
        error(`Failed to delete branch: ${e.message}`);
        process.exit(1);
      }
    });
}
