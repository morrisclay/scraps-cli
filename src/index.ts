#!/usr/bin/env node
import { Command } from "commander";
import { registerAuthCommands } from "./commands/auth.js";
import { registerStoreCommands } from "./commands/store.js";
import { registerRepoCommands } from "./commands/repo.js";
import { registerCommitCommand } from "./commands/commit.js";
import { registerBranchCommands } from "./commands/branch.js";
import { registerFileCommands } from "./commands/file.js";
import { registerTokenCommands } from "./commands/token.js";
import { registerCoordinateCommands } from "./commands/coordinate.js";
import { registerWatchCommand } from "./commands/watch.js";
import { loadConfig, saveConfig } from "./config.js";

const program = new Command();

program
  .name("scraps")
  .description("CLI for Scraps serverless Git")
  .version("0.1.5")
  .addHelpText("after", `
Getting Started:
  scraps signup                              # Create an account
  scraps login                               # Login with API key
  scraps store create mystore                # Create a store
  scraps repo create mystore/my-project      # Create a repository

Common Commands:
  scraps repo list                           # List all repositories
  scraps file tree mystore/my-project:main   # Browse files
  scraps file read mystore/my-project:main:README.md
  scraps commit mystore/my-project file.txt -b main -m "message"
  scraps watch mystore/my-project            # Stream live events

Multi-Agent Coordination:
  scraps coordinate status mystore/my-project:main
  scraps coordinate claim mystore/my-project:main "src/**" -m "Working on src"
  scraps coordinate watch mystore/my-project:main

For more info on a command, run: scraps <command> --help
`);

// Config command
program
  .command("config")
  .description("View or update configuration")
  .option("--host <host>", "Set default host")
  .option("--output <format>", "Set output format (table, json)")
  .option("--show", "Show current config")
  .addHelpText("after", `
Configuration is stored in ~/.scraps/config.json

Examples:
  scraps config                              # Show current config
  scraps config --show                       # Show current config
  scraps config --host https://custom.server # Set default server
  scraps config --output json                # Always output as JSON
  scraps config --output table               # Always output as table
`)
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
registerCommitCommand(program);
registerBranchCommands(program);
registerFileCommands(program);
registerTokenCommands(program);
registerCoordinateCommands(program);
registerWatchCommand(program);

program.parse();
