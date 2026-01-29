import { Command } from "commander";
import { existsSync } from "fs";
import { requireAuth } from "../api.js";
import { success, error, info } from "../utils/output.js";
import { gitExec, isGitRepo, buildAuthUrl } from "../utils/git.js";

function parseRepoRef(ref: string): { store: string; repo: string } {
  const parts = ref.split("/");
  if (parts.length !== 2) {
    throw new Error("Invalid repo reference. Use format: store/repo");
  }
  return { store: parts[0], repo: parts[1] };
}

export function registerInitCommand(program: Command): void {
  program
    .command("init <store/repo> [directory]")
    .description("Initialize a new repository (creates remote + local git repo)")
    .option("--no-create", "Don't create remote repo if it doesn't exist")
    .action(async (ref, directory, opts) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      const dir = directory || repo;

      // Check if directory exists and is already a git repo
      if (existsSync(dir) && isGitRepo(dir)) {
        error(`Directory '${dir}' is already a git repository`);
        process.exit(1);
      }

      // Try to get existing repo, or create it
      let repoExists = false;
      try {
        await client.get(`/api/v1/stores/${store}/repos/${repo}`);
        repoExists = true;
        info(`Repository ${store}/${repo} exists on server`);
      } catch (e: any) {
        // Server may return 403 or 404 for non-existent repos
        if ((e.status === 404 || e.status === 403) && opts.create !== false) {
          // Create the repo
          info(`Creating repository ${store}/${repo}...`);
          try {
            await client.post(`/api/v1/stores/${store}/repos`, { name: repo });
            repoExists = true;
            success(`Repository created on server`);
          } catch (createErr: any) {
            error(`Failed to create repository: ${createErr.message}`);
            process.exit(1);
          }
        } else if (e.status === 404 || e.status === 403) {
          error(`Repository ${store}/${repo} does not exist or you don't have access. Use --create to create it.`);
          process.exit(1);
        } else {
          error(`Failed to check repository: ${e.message}`);
          process.exit(1);
        }
      }

      // Create directory if needed
      if (!existsSync(dir)) {
        info(`Creating directory ${dir}...`);
        try {
          gitExec(["init", dir]);
        } catch (e: any) {
          error(`Failed to create directory: ${e.message}`);
          process.exit(1);
        }
      } else {
        // Initialize git in existing directory
        info(`Initializing git in ${dir}...`);
        try {
          gitExec(["init"], { cwd: dir });
        } catch (e: any) {
          error(`Failed to initialize git: ${e.message}`);
          process.exit(1);
        }
      }

      // Set up remote
      const authUrl = buildAuthUrl(
        client.getHost(),
        client.getApiKey(),
        store,
        repo
      );

      // Check if remote already exists
      try {
        gitExec(["remote", "get-url", "origin"], { cwd: dir });
        // Remote exists, update it
        info("Updating origin remote...");
        gitExec(["remote", "set-url", "origin", authUrl], { cwd: dir });
      } catch {
        // Remote doesn't exist, add it
        info("Adding origin remote...");
        gitExec(["remote", "add", "origin", authUrl], { cwd: dir });
      }

      // Clean URL for display (without credentials)
      const cleanUrl = `${client.getHost()}/stores/${store}/repos/${repo}`;

      success(`Initialized ${store}/${repo} in ${dir}`);
      info(`Remote: ${cleanUrl}`);
      info("");
      info("Next steps:");
      info("  1. Add files:    git add .");
      info("  2. Commit:       git commit -m 'Initial commit'");
      info("  3. Push:         scraps push");
    });
}
