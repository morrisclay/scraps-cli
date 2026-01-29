import { Command } from "commander";
import { requireAuth } from "../api.js";
import { error, info } from "../utils/output.js";
import { buildAuthUrl, gitClone } from "../utils/git.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    throw new Error("Invalid repo reference. Use format: store/repo");
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerCloneCommand(program: Command): void {
  program
    .command("clone <store/repo> [directory]")
    .description("Clone a repository")
    .action(async (ref, directory) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);

      // Verify repo exists
      try {
        await client.get(`/api/v1/stores/${store}/repos/${repo}`);
      } catch (e: any) {
        error(`Repository not found: ${e.message}`);
        process.exit(1);
      }

      const authUrl = buildAuthUrl(
        client.getHost(),
        client.getApiKey(),
        store,
        repo
      );

      info(`Cloning ${store}/${repo}...`);
      try {
        gitClone(authUrl, directory);
      } catch (e: any) {
        error(`Clone failed: ${e.message}`);
        process.exit(1);
      }
    });
}
