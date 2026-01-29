import { Command } from "commander";
import { requireAuth } from "../api.js";
import { error, info, color } from "../utils/output.js";
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
    .command("watch <store/repo>")
    .description("Watch repository events in real-time (commits, branch updates)")
    .option("-b, --branch <branch>", "Filter to specific branch")
    .option("--last-event <id>", "Resume from event ID")
    .action(async (ref, opts) => {
      const client = requireAuth();
      const { store, repo } = parseRepoRef(ref);
      const branch = opts.branch;

      // Build WebSocket URL
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
    });
}
