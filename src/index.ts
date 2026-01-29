#!/usr/bin/env node
import { Command } from "commander";
import { registerAuthCommands } from "./commands/auth.js";
import { registerStoreCommands } from "./commands/store.js";
import { registerRepoCommands } from "./commands/repo.js";
import { registerCloneCommand } from "./commands/clone.js";
import { registerFileCommands } from "./commands/file.js";
import { registerTokenCommands } from "./commands/token.js";
import { registerFightCommands } from "./commands/fight.js";
import { registerWatchCommand } from "./commands/watch.js";
import { registerStatusCommand } from "./commands/status.js";
import { loadConfig, saveConfig } from "./config.js";

const program = new Command();

program
  .name("scraps")
  .description("CLI for Scraps serverless Git")
  .version("0.2.4")
  .addHelpText("after", `
Getting Started:
  scraps signup                              # Create an account
  scraps login                               # Login with API key
  scraps status                              # Check login status
  scraps store create mystore                # Create a store
  scraps repo create mystore/my-project      # Create a repository
  scraps clone mystore/my-project            # Clone with git

Common Commands:
  scraps repo list                           # List all repositories
  scraps clone mystore/my-project            # Clone a repository
  scraps file tree mystore/my-project:main   # Browse files
  scraps file read mystore/my-project:main:README.md
  scraps watch mystore/my-project            # Stream live events

Fighting Over Scraps:
  scraps fight status mystore/my-project:main
  scraps fight claim mystore/my-project:main "src/**" -m "Working on src"
  scraps fight watch mystore/my-project:main

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
      console.log("Current configuration (~/.scraps/config.json):\n");
      console.log(`  Host:   ${config.default_host}`);
      console.log(`  Output: ${config.output_format}`);
      return;
    }

    const changes: string[] = [];

    if (opts.host) {
      config.default_host = opts.host;
      changes.push(`Host set to: ${opts.host}`);
    }
    if (opts.output) {
      if (opts.output !== "table" && opts.output !== "json") {
        console.error("Output format must be 'table' or 'json'");
        process.exit(1);
      }
      config.output_format = opts.output;
      changes.push(`Output format set to: ${opts.output}`);
    }

    saveConfig(config);
    changes.forEach((c) => console.log(`✓ ${c}`));
  });

// Register all command groups
registerAuthCommands(program);
registerStatusCommand(program);
registerStoreCommands(program);
registerRepoCommands(program);
registerCloneCommand(program);
registerFileCommands(program);
registerTokenCommands(program);
registerFightCommands(program);
registerWatchCommand(program);

program.parse();
