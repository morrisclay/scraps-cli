#!/usr/bin/env node
import { Command } from "commander";
import { registerAuthCommands } from "./commands/auth.js";
import { registerStoreCommands } from "./commands/store.js";
import { registerRepoCommands } from "./commands/repo.js";
import { registerCloneCommand } from "./commands/clone.js";
import { registerPushCommand } from "./commands/push.js";
import { registerPullCommand } from "./commands/pull.js";
import { registerCommitCommand } from "./commands/commit.js";
import { registerBranchCommands } from "./commands/branch.js";
import { registerFileCommands } from "./commands/file.js";
import { registerTokenCommands } from "./commands/token.js";
import { registerWebhookCommands } from "./commands/webhook.js";
import { registerCoordinateCommands } from "./commands/coordinate.js";
import { loadConfig, saveConfig } from "./config.js";

const program = new Command();

program
  .name("scraps")
  .description("CLI for Scraps serverless Git")
  .version("0.1.0");

// Config command
program
  .command("config")
  .description("View or update configuration")
  .option("--host <host>", "Set default host")
  .option("--output <format>", "Set output format (table, json)")
  .option("--show", "Show current config")
  .action((opts) => {
    const config = loadConfig();

    if (opts.show || (!opts.host && !opts.output)) {
      console.log(JSON.stringify(config, null, 2));
      return;
    }

    if (opts.host) {
      config.default_host = opts.host;
    }
    if (opts.output) {
      if (opts.output !== "table" && opts.output !== "json") {
        console.error("Output format must be 'table' or 'json'");
        process.exit(1);
      }
      config.output_format = opts.output;
    }

    saveConfig(config);
    console.log("Configuration updated");
  });

// Register all command groups
registerAuthCommands(program);
registerStoreCommands(program);
registerRepoCommands(program);
registerCloneCommand(program);
registerPushCommand(program);
registerPullCommand(program);
registerCommitCommand(program);
registerBranchCommands(program);
registerFileCommands(program);
registerTokenCommands(program);
registerWebhookCommands(program);
registerCoordinateCommands(program);

program.parse();
