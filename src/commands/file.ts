import { Command } from "commander";
import { requireAuth } from "../api.js";
import { error, output, outputTable, color } from "../utils/output.js";

// Parse store/repo:branch:path format
function parseFileRef(ref: string): {
  store: string;
  repo: string;
  branch: string;
  path: string;
} {
  // Format: store/repo:branch:path or store/repo:branch
  const colonParts = ref.split(":");
  if (colonParts.length < 2) {
    throw new Error("Invalid reference. Use format: store/repo:branch[:path]");
  }

  const repoRef = colonParts[0];
  const branch = colonParts[1];
  const path = colonParts.slice(2).join(":") || "";

  const slashParts = repoRef.split("/");
  if (slashParts.length !== 2) {
    throw new Error("Invalid repo reference. Use format: store/repo:branch[:path]");
  }

  return {
    store: slashParts[0],
    repo: slashParts[1],
    branch,
    path,
  };
}

export function registerFileCommands(program: Command): void {
  const file = program
    .command("file")
    .description("Read files from repositories")
    .addHelpText("after", `
Examples:
  scraps file read alice/my-project:main:README.md    # Read a file
  scraps file tree alice/my-project:main              # List root directory
  scraps file tree alice/my-project:main src/         # List subdirectory
`);

  file
    .command("read <store/repo:branch:path>")
    .description("Read a file from a repository")
    .addHelpText("after", `
Format: store/repo:branch:path

Examples:
  scraps file read alice/my-project:main:README.md
  scraps file read alice/my-project:feature/auth:src/index.ts
  scraps file read myteam/backend:v2.0:config/settings.json
`)
    .action(async (ref) => {
      const client = requireAuth();
      const { store, repo, branch, path } = parseFileRef(ref);

      if (!path) {
        error("Path is required. Use format: store/repo:branch:path");
        process.exit(1);
      }

      try {
        const result = await client.get(
          `/api/v1/stores/${store}/repos/${repo}/files/${encodeURIComponent(branch)}/${path}`
        );
        // Output raw content
        console.log(result.content);
      } catch (e: any) {
        error(`Failed to read file: ${e.message}`);
        process.exit(1);
      }
    });

  file
    .command("tree <store/repo:branch> [path]")
    .description("List files in a directory")
    .addHelpText("after", `
Format: store/repo:branch [path]

Examples:
  scraps file tree alice/my-project:main             # List root directory
  scraps file tree alice/my-project:main src/        # List src/ directory
  scraps file tree alice/my-project:feature/login components/
`)
    .action(async (ref, path) => {
      const client = requireAuth();
      const parsed = parseFileRef(ref + (path ? `:${path}` : ":"));
      const { store, repo, branch } = parsed;
      const treePath = path || parsed.path || "";

      try {
        const url = treePath
          ? `/api/v1/stores/${store}/repos/${repo}/tree/${encodeURIComponent(branch)}/${treePath}`
          : `/api/v1/stores/${store}/repos/${repo}/tree/${encodeURIComponent(branch)}`;
        const result = await client.get(url);
        const entries = result.entries || result.tree || result;

        if (Array.isArray(entries)) {
          output(entries, {
            headers: ["Type", "Name", "SHA"],
            rows: entries.map((entry: any) => [
              entry.type === "tree" ? color("dir", "blue") : "file",
              entry.name + (entry.type === "tree" ? "/" : ""),
              entry.sha?.slice(0, 7) || "",
            ]),
          });
        } else {
          output(result);
        }
      } catch (e: any) {
        error(`Failed to list tree: ${e.message}`);
        process.exit(1);
      }
    });

  program
    .command("log <store/repo:branch>")
    .description("Show commit history")
    .option("-n, --limit <count>", "Number of commits to show", "10")
    .addHelpText("after", `
Format: store/repo:branch

Examples:
  scraps log alice/my-project:main                   # Show last 10 commits
  scraps log alice/my-project:main -n 5              # Show last 5 commits
  scraps log alice/my-project:feature/auth --limit 20
`)
    .action(async (ref, opts) => {
      const client = requireAuth();
      const { store, repo, branch } = parseFileRef(ref + ":");

      try {
        const result = await client.get(
          `/api/v1/stores/${store}/repos/${repo}/log/${encodeURIComponent(branch)}?limit=${opts.limit}`
        );
        const commits = result.commits || result;

        if (Array.isArray(commits)) {
          for (const commit of commits) {
            console.log(
              color(commit.sha?.slice(0, 7) || commit.commit?.slice(0, 7), "yellow"),
              commit.message?.split("\n")[0] || ""
            );
            if (commit.author) {
              const authorStr = typeof commit.author === "string"
                ? commit.author
                : `${commit.author.name} <${commit.author.email}>`;
              console.log(
                color(`  Author: ${authorStr}`, "dim")
              );
            }
            if (commit.date || commit.timestamp) {
              const ts = commit.timestamp ? commit.timestamp * 1000 : commit.date;
              console.log(
                color(`  Date:   ${new Date(ts).toLocaleString()}`, "dim")
              );
            }
            console.log();
          }
        } else {
          output(commits);
        }
      } catch (e: any) {
        error(`Failed to get log: ${e.message}`);
        process.exit(1);
      }
    });
}
