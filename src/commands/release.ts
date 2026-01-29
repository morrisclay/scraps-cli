import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error } from "../utils/output.js";

function parseRepoRef(ref: string): { store: string; repo: string; branch?: string } {
  const colonIdx = ref.indexOf(":");
  let repoRef = ref;
  let branch: string | undefined;

  if (colonIdx !== -1) {
    repoRef = ref.slice(0, colonIdx);
    branch = ref.slice(colonIdx + 1);
  }

  const parts = repoRef.split("/");
  if (parts.length !== 2) {
    throw new Error("Invalid reference. Use format: store/repo:branch");
  }

  return { store: parts[0], repo: parts[1], branch };
}

export function registerReleaseCommand(program: Command): void {
  program
    .command("release <store/repo:branch> <patterns...>")
    .description("Release claimed file patterns")
    .requiredOption("--agent-id <id>", "Agent ID that owns the claims")
    .addHelpText("after", `
Release file patterns you previously claimed.
You must provide the same agent ID that was used when claiming.

Format: store/repo:branch <patterns...> --agent-id <id>

Examples:
  scraps release alice/my-project:main "src/**" --agent-id cli-abc123
  scraps release myteam/backend:main "api/**" "docs/**" --agent-id my-agent
`)
    .action(async (ref, patterns, opts) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      try {
        await client.delete(
          `/stores/${store}/repos/${repo}/branches/${encodeURIComponent(branch)}/coordinate/claim`,
          {
            agent_id: opts.agentId,
            patterns,
          }
        );
        success(`Released patterns: ${patterns.join(", ")}`);
      } catch (e: any) {
        error(`Failed to release: ${e.message}`);
        process.exit(1);
      }
    });
}
