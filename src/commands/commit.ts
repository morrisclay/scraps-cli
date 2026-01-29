import { Command } from "commander";
import { readFileSync, statSync } from "fs";
import { basename } from "path";
import { requireAuth } from "../api.js";
import { success, error, info, output } from "../utils/output.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    throw new Error("Invalid repo reference. Use format: store/repo");
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerCommitCommand(program: Command): void {
  program
    .command("commit <store/repo> <files...>")
    .description("Create a commit via API (no local git needed)")
    .requiredOption("-m, --message <message>", "Commit message")
    .requiredOption("-b, --branch <branch>", "Target branch")
    .option("-a, --author <name>", "Author name")
    .option("-e, --email <email>", "Author email")
    .option("--base <sha>", "Base commit SHA (for optimistic locking)")
    .addHelpText("after", `
Format: store/repo <files...> -b branch -m message

This command creates commits directly via the API without needing local git.
Perfect for agents and automation workflows.

Examples:
  scraps commit alice/my-project README.md -b main -m "Update readme"
  scraps commit alice/my-project src/app.ts src/utils.ts -b feature/auth -m "Add auth"
  scraps commit myteam/backend config.json -b main -m "Update config" --base abc1234

Options explained:
  -b, --branch    Required. The branch to commit to
  -m, --message   Required. The commit message
  -a, --author    Optional. Override author name
  -e, --email     Optional. Override author email
  --base          Optional. Expect this SHA as the current HEAD (for conflict detection)
`)
    .action(async (ref, files, opts) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);

      // Read file contents
      const fileContents: { path: string; content: string }[] = [];
      for (const file of files) {
        try {
          const stat = statSync(file);
          if (!stat.isFile()) {
            error(`Not a file: ${file}`);
            process.exit(1);
          }
          const content = readFileSync(file, "utf-8");
          fileContents.push({
            path: basename(file),
            content,
          });
        } catch (e: any) {
          error(`Cannot read file ${file}: ${e.message}`);
          process.exit(1);
        }
      }

      const body: any = {
        branch: opts.branch,
        message: opts.message,
        files: fileContents,
      };

      if (opts.author) body.author_name = opts.author;
      if (opts.email) body.author_email = opts.email;
      if (opts.base) body.base_commit = opts.base;

      info(`Creating commit on ${store}/${repo}:${opts.branch}...`);
      try {
        const result = await client.post(
          `/api/v1/stores/${store}/repos/${repo}/commits`,
          body
        );
        success(`Commit created: ${result.sha?.slice(0, 7) || result.commit_sha?.slice(0, 7)}`);
        output(result);
      } catch (e: any) {
        error(`Commit failed: ${e.message}`);
        process.exit(1);
      }
    });
}
