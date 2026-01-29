import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, info, output, color, formatDateTime } from "../utils/output.js";
import { buildWsUrl, connectWebSocket } from "../utils/websocket.js";
import { randomUUID } from "crypto";

function parseRepoRef(ref: string): { store: string; repo: string; branch?: string } {
  // Format: store/repo or store/repo:branch
  const colonIdx = ref.indexOf(":");
  let repoRef = ref;
  let branch: string | undefined;

  if (colonIdx !== -1) {
    repoRef = ref.slice(0, colonIdx);
    branch = ref.slice(colonIdx + 1);
  }

  const parts = repoRef.split("/");
  if (parts.length !== 2) {
    throw new Error("Invalid reference. Use format: store/repo[:branch]");
  }

  return { store: parts[0], repo: parts[1], branch };
}

export function registerCoordinateCommands(program: Command): void {
  const coordinate = program
    .command("coordinate")
    .description("Multi-agent coordination for preventing edit conflicts")
    .addHelpText("after", `
Multi-agent coordination allows multiple agents to claim file patterns,
preventing conflicts when working on the same repository.

Format: store/repo:branch

Examples:
  scraps coordinate status alice/my-project:main
  scraps coordinate claim alice/my-project:main "src/**" -m "Refactoring src"
  scraps coordinate release alice/my-project:main "src/**" --agent-id cli-abc123
  scraps coordinate watch alice/my-project:main
`);

  coordinate
    .command("status <store/repo:branch>")
    .description("Show coordination status (active claims, agents, recent activity)")
    .addHelpText("after", `
Format: store/repo:branch

Examples:
  scraps coordinate status alice/my-project:main
  scraps coordinate status myteam/backend:feature/auth
`)
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      try {
        const status = await client.get(
          `/stores/${store}/repos/${repo}/branches/${encodeURIComponent(branch)}/coordinate/state`
        );

        console.log(color(`Coordination status for ${store}/${repo}:${branch}`, "bold"));
        console.log();

        if (status.intents && status.intents.length > 0) {
          console.log(color("Active Claims:", "cyan"));
          for (const intent of status.intents) {
            console.log(
              `  ${color(intent.agent_id?.slice(0, 8) || "unknown", "yellow")} - ${intent.patterns?.join(", ") || "(no patterns)"}`
            );
            if (intent.claim) {
              console.log(color(`    "${intent.claim}"`, "dim"));
            }
            if (intent.created_at) {
              console.log(color(`    Claimed: ${formatDateTime(new Date(intent.created_at))}`, "dim"));
            }
          }
        } else {
          console.log(color("No active claims", "dim"));
        }

        if (status.presence && status.presence.length > 0) {
          console.log();
          console.log(color("Active Agents:", "cyan"));
          for (const p of status.presence) {
            console.log(
              `  ${color(p.agent_name || p.agent_id?.slice(0, 8) || "unknown", "yellow")} - watching: ${p.active_paths?.join(", ") || "(none)"}`
            );
          }
        }

        if (status.activity && status.activity.length > 0) {
          console.log();
          console.log(color("Recent Activity:", "cyan"));
          for (const act of status.activity.slice(-5)) {
            console.log(
              `  ${formatDateTime(new Date(act.timestamp))} - ${act.type}: ${act.claim || act.patterns?.join(", ") || ""}`
            );
          }
        }
      } catch (e: any) {
        error(`Failed to get status: ${e.message}`);
        process.exit(1);
      }
    });

  coordinate
    .command("claim <store/repo:branch> <patterns...>")
    .description("Claim file patterns for editing")
    .option("-m, --message <message>", "Description of planned changes", "CLI claim")
    .option("--agent-id <id>", "Agent ID (auto-generated if not provided)")
    .option("--ttl <seconds>", "Claim TTL in seconds", "300")
    .addHelpText("after", `
Format: store/repo:branch <patterns...>

Patterns use glob syntax (e.g., "src/**", "*.ts", "docs/api/*").
Claims prevent other agents from claiming overlapping patterns.

Examples:
  scraps coordinate claim alice/my-project:main "src/**" -m "Refactoring"
  scraps coordinate claim alice/my-project:main "*.ts" "*.tsx" -m "Type fixes"
  scraps coordinate claim myteam/backend:main "api/**" --ttl 600 -m "API update"
  scraps coordinate claim alice/my-project:main "README.md" --agent-id my-agent-123

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

  coordinate
    .command("release <store/repo:branch> <patterns...>")
    .description("Release claimed file patterns")
    .requiredOption("--agent-id <id>", "Agent ID that owns the claims")
    .addHelpText("after", `
Format: store/repo:branch <patterns...> --agent-id <id>

Release patterns you previously claimed. You must provide the same agent ID
that was used when claiming.

Examples:
  scraps coordinate release alice/my-project:main "src/**" --agent-id cli-abc123
  scraps coordinate release myteam/backend:main "api/**" "docs/**" --agent-id my-agent
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

  coordinate
    .command("watch <store/repo:branch>")
    .description("Watch coordination activity in real-time via WebSocket")
    .addHelpText("after", `
Format: store/repo:branch

Streams live coordination events: claims, releases, presence updates.
Press Ctrl+C to stop.

Examples:
  scraps coordinate watch alice/my-project:main
  scraps coordinate watch myteam/backend:feature/auth
`)
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      // Build WebSocket URL for tail endpoint
      const wsHost = client.getHost().replace(/^http/, "ws");
      const wsUrl = `${wsHost}/stores/${store}/repos/${repo}/branches/${encodeURIComponent(branch)}/coordinate/tail?token=${client.getApiKey()}`;

      info(`Watching ${store}/${repo}:${branch}...`);
      console.log(color("Press Ctrl+C to stop", "dim"));
      console.log();

      const ws = connectWebSocket(wsUrl, {
        onOpen: () => {
          console.log(color("Connected", "green"));
        },
        onMessage: (msg) => {
          const timestamp = new Date().toLocaleTimeString();
          const eventType = color(msg.type || "event", "cyan");

          if (msg.type === "activity" && msg.activity) {
            const act = msg.activity;
            console.log(
              `[${timestamp}] ${color(act.type, "cyan")} ${act.agent_id?.slice(0, 8) || ""}: ${act.claim || act.patterns?.join(", ") || ""}`
            );
          } else if (msg.type === "intent_update") {
            console.log(`[${timestamp}] ${eventType} - ${msg.intents?.length || 0} active claims`);
          } else if (msg.type === "presence_update") {
            console.log(`[${timestamp}] ${eventType} - ${msg.presence?.length || 0} agents online`);
          } else {
            console.log(`[${timestamp}] ${eventType}`, JSON.stringify(msg));
          }
        },
        onError: (err) => {
          error(`WebSocket error: ${err.message}`);
        },
        onClose: () => {
          console.log(color("Disconnected", "yellow"));
          process.exit(0);
        },
      });

      // Handle Ctrl+C
      process.on("SIGINT", () => {
        ws.close();
      });
    });
}
