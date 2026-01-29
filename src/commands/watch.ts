import { Command } from "commander";
import { requireAuth } from "../api.js";
import { error, info, color, formatDateTime } from "../utils/output.js";
import { connectWebSocket } from "../utils/websocket.js";

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
    throw new Error("Invalid reference. Use format: store/repo[:branch]");
  }

  return { store: parts[0], repo: parts[1], branch };
}

export function registerWatchCommand(program: Command): void {
  program
    .command("watch <store/repo[:branch]>")
    .description("Watch repository events in real-time")
    .option("-b, --branch <branch>", "Filter to specific branch")
    .option("--last-event <id>", "Resume from event ID")
    .option("--claims", "Watch claim activity instead of repo events (requires branch)")
    .addHelpText("after", `
Streams live events via WebSocket. Press Ctrl+C to stop.

Format: store/repo or store/repo:branch

Repository events (default):
  scraps watch alice/my-project                     # Watch all branches
  scraps watch alice/my-project -b main             # Watch only main branch
  scraps watch alice/my-project --last-event 42     # Resume from event ID

Claim activity (--claims):
  scraps watch alice/my-project:main --claims       # Watch claim/release activity

Event types:
  Repository: commit, branch:create, branch:delete, branch:update, ref:update
  Claims: claim, release, intent_update, presence_update
`)
    .action(async (ref, opts) => {
      const client = requireAuth();
      const { store, repo, branch: refBranch } = parseRepoRef(ref);
      const branch = refBranch || opts.branch;

      if (opts.claims) {
        // Watch coordination/claim activity
        if (!branch) {
          error("Branch is required for --claims. Use format: store/repo:branch");
          process.exit(1);
        }

        const wsHost = client.getHost().replace(/^http/, "ws");
        const wsUrl = `${wsHost}/stores/${store}/repos/${repo}/branches/${encodeURIComponent(branch)}/coordinate/tail?token=${client.getApiKey()}`;

        info(`Watching claims on ${store}/${repo}:${branch}...`);
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

        process.on("SIGINT", () => {
          ws.close();
        });
      } else {
        // Watch repository events
        const wsHost = client.getHost().replace(/^http/, "ws");
        const params = new URLSearchParams({ token: client.getApiKey() });
        if (branch) params.set("branch", branch);
        if (opts.lastEvent) params.set("lastEventId", opts.lastEvent);

        const wsUrl = `${wsHost}/stores/${store}/repos/${repo}/ws?${params}`;

        info(`Watching ${store}/${repo}${branch ? `:${branch}` : ""}...`);
        console.log(color("Press Ctrl+C to stop", "dim"));
        console.log();

        const ws = connectWebSocket(wsUrl, {
          onOpen: () => {
            console.log(color("Connected", "green"));
          },
          onMessage: (msg) => {
            const timestamp = new Date().toLocaleTimeString();

            if (msg.type === "commit") {
              console.log(
                `[${timestamp}] ${color("commit", "green")} ${color(msg.sha?.slice(0, 7) || "", "yellow")} ${msg.message || ""}`
              );
              if (msg.branch) {
                console.log(color(`         on ${msg.branch}`, "dim"));
              }
              if (msg.files && msg.files.length > 0) {
                for (const f of msg.files.slice(0, 5)) {
                  const icon = f.action === "add" ? "+" : f.action === "delete" ? "-" : "~";
                  console.log(color(`         ${icon} ${f.path}`, "dim"));
                }
                if (msg.files.length > 5) {
                  console.log(color(`         ... and ${msg.files.length - 5} more`, "dim"));
                }
              }
            } else if (msg.type === "branch:create") {
              console.log(
                `[${timestamp}] ${color("branch:create", "cyan")} ${msg.branch || msg.name}`
              );
            } else if (msg.type === "branch:delete") {
              console.log(
                `[${timestamp}] ${color("branch:delete", "red")} ${msg.branch || msg.name}`
              );
            } else if (msg.type === "branch:update") {
              console.log(
                `[${timestamp}] ${color("branch:update", "blue")} ${msg.branch || msg.name} ${msg.oldSha?.slice(0, 7) || ""} -> ${msg.newSha?.slice(0, 7) || ""}`
              );
            } else if (msg.type === "ref:update") {
              console.log(
                `[${timestamp}] ${color("ref:update", "blue")} ${msg.ref} -> ${msg.sha?.slice(0, 7) || ""}`
              );
            } else {
              console.log(`[${timestamp}] ${color(msg.type || "event", "magenta")}`, JSON.stringify(msg));
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

        process.on("SIGINT", () => {
          ws.close();
        });
      }
    });
}
