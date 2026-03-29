import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import path from "node:path";

const projectDir = process.cwd();
const mode = process.argv[2] ?? "dev";
const extraArgs = process.argv.slice(3);

const requireFromProject = createRequire(path.join(projectDir, "package.json"));
const { loadEnvConfig } = requireFromProject("@next/env");
const nextBin = requireFromProject.resolve("next/dist/bin/next");

loadEnvConfig(projectDir, mode === "dev");

const args = [nextBin, mode];
const hasExplicitPort = extraArgs.includes("-p") || extraArgs.includes("--port");
if ((mode === "dev" || mode === "start") && !hasExplicitPort) {
  const port = String(process.env.PORT ?? "").trim();
  if (port) {
    args.push("-p", port);
  }
}
args.push(...extraArgs);

const child = spawn(process.execPath, args, {
  cwd: projectDir,
  env: process.env,
  stdio: "inherit",
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
