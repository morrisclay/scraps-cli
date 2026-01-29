import { Command } from "commander";
import { requireAuth } from "../api.js";
import { error, info } from "../utils/output.js";
import {
  isGitRepo,
  getCurrentBranch,
  getRemoteUrl,
  parseRepoUrl,
  buildAuthUrl,
  gitPush,
  gitExec,
} from "../utils/git.js";

export function registerPushCommand(program: Command): void {
  program
    .command("push")
    .description("Push current branch to remote")
    .option("-r, --remote <remote>", "Remote name", "origin")
    .option("-b, --branch <branch>", "Branch to push")
    .option("-u, --set-upstream", "Set upstream tracking")
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

      // Get remote URL and check if it's a scraps repo
      const remoteUrl = getRemoteUrl(opts.remote);
      if (!remoteUrl) {
        error(`Remote '${opts.remote}' not found`);
        process.exit(1);
      }

      const parsed = parseRepoUrl(remoteUrl);
      if (!parsed) {
        // Not a scraps URL, just run regular git push
        info(`Pushing to ${opts.remote}...`);
        gitPush(opts.remote, branch);
        return;
      }

      // Update remote URL with current credentials
      const authUrl = buildAuthUrl(
        parsed.host,
        client.getApiKey(),
        parsed.store,
        parsed.repo
      );

      // Temporarily set the push URL with auth
      try {
        gitExec(["remote", "set-url", "--push", opts.remote, authUrl]);
        info(`Pushing ${branch} to ${parsed.store}/${parsed.repo}...`);

        const args = ["push", opts.remote, branch];
        if (opts.setUpstream) args.push("-u");
        gitExec(args, { stdio: "inherit" });
      } finally {
        // Reset push URL to non-auth version
        const cleanUrl = `${parsed.host}/stores/${parsed.store}/repos/${parsed.repo}`;
        gitExec(["remote", "set-url", "--push", opts.remote, cleanUrl]);
      }
    });
}
