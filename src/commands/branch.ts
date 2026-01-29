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

export function registerBranchCommands(program: Command): void {
  const branch = program.command("branch").description("Manage branches");

  branch
    .command("list <store/repo>")
    .description("List branches in a repository")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      try {
        const branches = await client.get(
          `/api/v1/stores/${store}/repos/${repo}/branches`
        );
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
    .option("-f, --from <branch>", "Branch to create from", "main")
    .action(async (ref, name, opts) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      try {
        await client.post(`/api/v1/stores/${store}/repos/${repo}/branches`, {
          name,
          from: opts.from,
        });
        success(`Branch '${name}' created from '${opts.from}'`);
      } catch (e: any) {
        error(`Failed to create branch: ${e.message}`);
        process.exit(1);
      }
    });

  branch
    .command("delete <store/repo> <name>")
    .description("Delete a branch")
    .action(async (ref, name) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      try {
        await client.delete(
          `/api/v1/stores/${store}/repos/${repo}/branches/${name}`
        );
        success(`Branch '${name}' deleted`);
      } catch (e: any) {
        error(`Failed to delete branch: ${e.message}`);
        process.exit(1);
      }
    });
}
