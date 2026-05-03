import { execSync } from "child_process";
import * as path from "path";

const root = path.resolve(__dirname, "..");
const files = [
  "internal/config/config.go",
  "internal/config/profiles.go",
  "cmd/devon/main.go",
  "proto/devon.proto",
  "internal/headless/types.go",
  "internal/headless/server.go",
  "internal/headless/handler.go",
  "internal/headless/server_test.go",
  "devon.toml",
  ".task-state.json",
].map((f) => path.join(root, f));

try {
  execSync(`git -C ${root} add ${files.join(" ")}`, { stdio: "inherit" });
  console.log("Files staged successfully.");
} catch (err) {
  console.error("Failed to stage files:", err.message);
  process.exit(1);
}
