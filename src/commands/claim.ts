import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, info, color, formatDateTime } from "../utils/output.js";
import { randomUUID } from "crypto";

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

export function registerClaimCommand(program: Command): void {
  program
    .command("claim <store/repo:branch> <patterns...>")
    .description("Claim file patterns for editing (multi-agent coordination)")
    .option("-m, --message <message>", "Description of planned changes", "CLI claim")
    .option("--agent-id <id>", "Agent ID (auto-generated if not provided)")
    .option("--ttl <seconds>", "Claim TTL in seconds", "300")
    .addHelpText("after", `
Claim file patterns to prevent other agents from editing them.
Patterns use glob syntax (e.g., "src/**", "*.ts", "docs/api/*").

Format: store/repo:branch <patterns...>

Examples:
  scraps claim alice/my-project:main "src/**" -m "Refactoring"
  scraps claim alice/my-project:main "*.ts" "*.tsx" -m "Type fixes"
  scraps claim myteam/backend:main "api/**" --ttl 600 -m "API update"
  scraps claim alice/my-project:main "README.md" --agent-id my-agent-123

Options:
  -m, --message   Description shown to other agents (default: "CLI claim")
  --agent-id      Your agent ID (auto-generated if not provided, save for release)
  --ttl           Time-to-live in seconds before auto-release (default: 300)
`)
    .action(async (ref, patterns, opts) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      const agentId = opts.agentId || `cli-${randomUUID().slice(0, 8)}`;

      try {
        const result = await client.post(
          `/stores/${store}/repos/${repo}/branches/${encodeURIComponent(branch)}/coordinate/claim`,
          {
            agent_id: agentId,
            patterns,
            claim: opts.message,
            ttl_seconds: parseInt(opts.ttl),
          }
        );

        if (result.type === "claim_conflict") {
          error("Claim conflict detected:");
          for (const c of result.conflicts || []) {
            console.log(`  ${color(c.agent_name || c.agent_id?.slice(0, 8), "yellow")}: ${c.patterns?.join(", ")} - "${c.claim}"`);
          }
          process.exit(1);
        }

        success(`Claimed patterns: ${patterns.join(", ")}`);
        info(`Agent ID: ${agentId}`);
        if (result.expires_at) {
          info(`Expires: ${formatDateTime(new Date(result.expires_at))}`);
        }
      } catch (e: any) {
        error(`Failed to claim: ${e.message}`);
        process.exit(1);
      }
    });
}
