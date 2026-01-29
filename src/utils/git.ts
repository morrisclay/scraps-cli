import { execSync, spawnSync } from "child_process";

export function gitExec(
  args: string[],
  options: { cwd?: string; stdio?: "inherit" | "pipe" } = {}
): string {
  const result = spawnSync("git", args, {
    cwd: options.cwd,
    stdio: options.stdio || "pipe",
    encoding: "utf-8",
  });

  if (result.error) {
    throw result.error;
  }

  if (result.status !== 0) {
    const stderr = result.stderr?.trim() || "git command failed";
    throw new Error(stderr);
  }

  return result.stdout?.trim() || "";
}

export function isGitRepo(dir: string = "."): boolean {
  try {
    gitExec(["rev-parse", "--git-dir"], { cwd: dir });
    return true;
  } catch {
    return false;
  }
}

export function getCurrentBranch(dir: string = "."): string | null {
  try {
    return gitExec(["rev-parse", "--abbrev-ref", "HEAD"], { cwd: dir });
  } catch {
    return null;
  }
}

export function getRemoteUrl(remote: string = "origin", dir: string = "."): string | null {
  try {
    return gitExec(["remote", "get-url", remote], { cwd: dir });
  } catch {
    return null;
  }
}

export function parseRepoUrl(url: string): { host: string; store: string; repo: string } | null {
  // Match patterns like:
  // https://x:key@host/stores/slug/repos/name
  // http://host/stores/slug/repos/name
  const match = url.match(
    /(https?):\/\/(?:[^@]+@)?([^/]+)\/stores\/([^/]+)\/repos\/([^/.]+)/
  );
  if (!match) return null;
  return { host: `${match[1]}://${match[2]}`, store: match[3], repo: match[4] };
}

export function buildAuthUrl(host: string, apiKey: string, store: string, repo: string): string {
  const hostUrl = new URL(host);
  return `${hostUrl.protocol}//x:${apiKey}@${hostUrl.host}/stores/${store}/repos/${repo}`;
}

export function gitClone(url: string, dir?: string): void {
  const args = ["clone", url];
  if (dir) args.push(dir);
  execSync(`git ${args.join(" ")}`, { stdio: "inherit" });
}

export function gitPush(remote: string = "origin", branch?: string, dir: string = "."): void {
  const args = ["push", remote];
  if (branch) args.push(branch);
  execSync(`git ${args.join(" ")}`, { stdio: "inherit", cwd: dir });
}

export function gitPull(remote: string = "origin", branch?: string, dir: string = "."): void {
  const args = ["pull", remote];
  if (branch) args.push(branch);
  execSync(`git ${args.join(" ")}`, { stdio: "inherit", cwd: dir });
}

export function gitFetch(remote: string = "origin", dir: string = "."): void {
  execSync(`git fetch ${remote}`, { stdio: "inherit", cwd: dir });
}
