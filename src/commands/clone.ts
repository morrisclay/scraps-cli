import { Command } from "commander";
import { execSync } from "child_process";
import { requireAuth } from "../api.js";
import { error, info } from "../utils/output.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    error(`Invalid format: "${ref}"`);
    info("Expected format: <store>/<repo>");
    info("Example: scraps clone alice/my-project");
    process.exit(1);
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerCloneCommand(program: Command): void {
  program
    .command("clone <store/repo> [directory]")
    .description("Clone a repository using git")
    .option("--url-only", "Print the clone URL without cloning")
    .addHelpText("after", `
Clones a Scraps repository using your saved credentials.
This is a convenience wrapper around git clone.

Examples:
  scraps clone alice/my-project              # Clone to ./my-project
  scraps clone alice/my-project mydir        # Clone to ./mydir
  scraps clone alice/my-project --url-only   # Just print the URL
`)
    .action(async (ref, directory, opts) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);

      // Construct authenticated git URL
      const host = client.getHost().replace(/^https?:\/\//, "");
      const protocol = client.getHost().startsWith("https") ? "https" : "http";
      const cloneUrl = `${protocol}://x:${client.getApiKey()}@${host}/stores/${store}/repos/${repo}`;

      if (opts.urlOnly) {
        // Print the full URL (can be piped to git clone)
        console.log(cloneUrl);
        return;
      }

      const targetDir = directory || repo;
      info(`Cloning ${store}/${repo} into ${targetDir}...`);

      try {
        execSync(`git clone ${cloneUrl} ${targetDir}`, {
          stdio: "inherit",
        });
      } catch (e: any) {
        // git clone already prints errors
        process.exit(1);
      }
    });
}
