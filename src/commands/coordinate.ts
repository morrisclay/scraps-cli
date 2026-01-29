import { Command } from "commander";
import { requireAuth } from "../api.js";
import { success, error, info, output, color, formatDateTime } from "../utils/output.js";
import { buildWsUrl, connectWebSocket } from "../utils/websocket.js";

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
    .description("Multi-agent coordination");

  coordinate
    .command("status <store/repo:branch>")
    .description("Show coordination status")
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      try {
        const status = await client.get(
          `/api/v1/stores/${store}/repos/${repo}/branches/${branch}/coordinate`
        );

        console.log(color(`Coordination status for ${store}/${repo}:${branch}`, "bold"));
        console.log();

        if (status.claims && status.claims.length > 0) {
          console.log(color("Active Claims:", "cyan"));
          for (const claim of status.claims) {
            console.log(
              `  ${color(claim.agent_id?.slice(0, 8) || "unknown", "yellow")} - ${claim.patterns?.join(", ") || "(no patterns)"}`
            );
            if (claim.message) {
              console.log(color(`    "${claim.message}"`, "dim"));
            }
            if (claim.timestamp) {
              console.log(color(`    Claimed: ${formatDateTime(claim.timestamp)}`, "dim"));
            }
          }
        } else {
          console.log(color("No active claims", "dim"));
        }

        if (status.activity && status.activity.length > 0) {
          console.log();
          console.log(color("Recent Activity:", "cyan"));
          for (const act of status.activity.slice(0, 5)) {
            console.log(
              `  ${formatDateTime(act.timestamp)} - ${act.type}: ${act.summary || act.message || ""}`
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
    .option("-m, --message <message>", "Description of planned changes")
    .action(async (ref, patterns, opts) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      try {
        await client.post(
          `/api/v1/stores/${store}/repos/${repo}/branches/${branch}/coordinate/claim`,
          {
            patterns,
            message: opts.message,
          }
        );
        success(`Claimed patterns: ${patterns.join(", ")}`);
      } catch (e: any) {
        error(`Failed to claim: ${e.message}`);
        process.exit(1);
      }
    });

  coordinate
    .command("release <store/repo:branch> [patterns...]")
    .description("Release claimed file patterns")
    .option("--all", "Release all claims")
    .action(async (ref, patterns, opts) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      if (!opts.all && (!patterns || patterns.length === 0)) {
        error("Specify patterns to release or use --all");
        process.exit(1);
      }

      try {
        await client.post(
          `/api/v1/stores/${store}/repos/${repo}/branches/${branch}/coordinate/release`,
          {
            patterns: opts.all ? undefined : patterns,
            all: opts.all,
          }
        );
        success(opts.all ? "All claims released" : `Released patterns: ${patterns.join(", ")}`);
      } catch (e: any) {
        error(`Failed to release: ${e.message}`);
        process.exit(1);
      }
    });

  coordinate
    .command("watch <store/repo:branch>")
    .description("Watch coordination activity in real-time")
    .option("--last-event <id>", "Resume from event ID")
    .action(async (ref, opts) => {
      const client = requireAuth();
      const { store, repo, branch } = parseRepoRef(ref);

      if (!branch) {
        error("Branch is required. Use format: store/repo:branch");
        process.exit(1);
      }

      const wsUrl = buildWsUrl(
        client.getHost(),
        store,
        repo,
        client.getApiKey(),
        {
          branch,
          lastEventId: opts.lastEvent ? parseInt(opts.lastEvent) : undefined,
        }
      );

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

          if (msg.type === "commit") {
            console.log(
              `[${timestamp}] ${eventType} ${color(msg.sha?.slice(0, 7) || "", "yellow")} - ${msg.message || ""}`
            );
          } else if (msg.type?.startsWith("branch:")) {
            console.log(
              `[${timestamp}] ${eventType} ${msg.branch || msg.name || ""}`
            );
          } else if (msg.type === "claim" || msg.type === "release") {
            console.log(
              `[${timestamp}] ${eventType} ${msg.agent_id?.slice(0, 8) || ""}: ${msg.patterns?.join(", ") || ""}`
            );
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
