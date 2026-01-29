import { Command } from "commander";
import { requireAuth } from "../api.js";
import { error, info } from "../utils/output.js";
import {
  isGitRepo,
  getCurrentBranch,
  getRemoteUrl,
  parseRepoUrl,
  buildAuthUrl,
  gitPull,
  gitFetch,
  gitExec,
} from "../utils/git.js";

export function registerPullCommand(program: Command): void {
  program
    .command("pull")
    .description("Pull current branch from remote")
    .option("-r, --remote <remote>", "Remote name", "origin")
    .option("-b, --branch <branch>", "Branch to pull")
    .action(async (opts) => {
      if (!isGitRepo()) {
        error("Not a git repository");
        process.exit(1);
      }

      const client = requireAuth();
      const branch = opts.branch || getCurrentBranch();
      if (!branch) {
        error("Could not determine current branch");
        process.exit(1);
      }

      const remoteUrl = getRemoteUrl(opts.remote);
      if (!remoteUrl) {
        error(`Remote '${opts.remote}' not found`);
        process.exit(1);
      }

      const parsed = parseRepoUrl(remoteUrl);
      if (!parsed) {
        info(`Pulling from ${opts.remote}...`);
        gitPull(opts.remote, branch);
        return;
      }

      const authUrl = buildAuthUrl(
        parsed.host,
        client.getApiKey(),
        parsed.store,
        parsed.repo
      );

      try {
        gitExec(["remote", "set-url", opts.remote, authUrl]);
        info(`Pulling ${branch} from ${parsed.store}/${parsed.repo}...`);
        gitExec(["pull", opts.remote, branch], { stdio: "inherit" });
      } finally {
        const cleanUrl = `${parsed.host}/stores/${parsed.store}/repos/${parsed.repo}`;
        gitExec(["remote", "set-url", opts.remote, cleanUrl]);
      }
    });

  program
    .command("fetch")
    .description("Fetch from remote without merging")
    .option("-r, --remote <remote>", "Remote name", "origin")
    .action(async (opts) => {
      if (!isGitRepo()) {
        error("Not a git repository");
        process.exit(1);
      }

      const client = requireAuth();
      const remoteUrl = getRemoteUrl(opts.remote);
      if (!remoteUrl) {
        error(`Remote '${opts.remote}' not found`);
        process.exit(1);
      }

      const parsed = parseRepoUrl(remoteUrl);
      if (!parsed) {
        info(`Fetching from ${opts.remote}...`);
        gitFetch(opts.remote);
        return;
      }

      const authUrl = buildAuthUrl(
        parsed.host,
        client.getApiKey(),
        parsed.store,
        parsed.repo
      );

      try {
        gitExec(["remote", "set-url", opts.remote, authUrl]);
        info(`Fetching from ${parsed.store}/${parsed.repo}...`);
        gitExec(["fetch", opts.remote], { stdio: "inherit" });
      } finally {
        const cleanUrl = `${parsed.host}/stores/${parsed.store}/repos/${parsed.repo}`;
        gitExec(["remote", "set-url", opts.remote, cleanUrl]);
      }
    });
}
