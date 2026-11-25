import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

const version = process.argv[2] || "0.1.0-dev.1";
if (!/^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(version)) throw Error("Invalid version");
const out = path.resolve(process.argv[3] || "dist-observer");
const publishable = process.argv[4] === "--publishable";
if (publishable && !/^\d+\.\d+\.\d+-(?:alpha|beta|rc|dev)\.[0-9]+$/.test(version)) throw Error("Preview publisher only accepts explicit prerelease versions");
if (publishable) await fs.access("LICENSE"); // maintainers must choose a redistribution license first
const pkg = path.join(out, "npm");
const platforms = [
  ["darwin", "arm64", "darwin", "arm64"],
  ["darwin", "amd64", "darwin", "x64"],
  ["linux", "amd64", "linux", "x64"]
];
await fs.mkdir(out, { recursive: true });
await fs.mkdir(pkg); // refuse to overwrite a prior build
const manifest = { schemaVersion: 1, version, sessionSchemaVersions: [1], platforms: {} };
for (const [goos, goarch, os, arch] of platforms) {
  const target = os + "-" + arch;
  const relative = "bin/" + target + "/c-observer";
  const binary = path.join(pkg, relative);
  await fs.mkdir(path.dirname(binary), { recursive: true });
  execFileSync("go", ["build", "-trimpath", "-ldflags", "-s -w -X main.version=" + version, "-o", binary, "./cmd/c-observer"], {
    stdio: "inherit", env: { ...process.env, CGO_ENABLED: "0", GOOS: goos, GOARCH: goarch }
  });
  await fs.chmod(binary, 0o755);
  const sha256 = createHash("sha256").update(await fs.readFile(binary)).digest("hex");
  manifest.platforms[target] = { executable: relative, sha256, status: "preview" };
  const dir = path.join(out, "standalone-" + target);
  await fs.mkdir(dir);
  await fs.copyFile(binary, path.join(dir, "c-observer"));
  await fs.chmod(path.join(dir, "c-observer"), 0o755);
  await fs.copyFile("docs/integration.md", path.join(dir, "integration.md"));
  if (publishable) await fs.copyFile("LICENSE",path.join(dir,"LICENSE"));
  await fs.writeFile(path.join(dir, "README.txt"), "C Observer " + version + "\nRun ./c-observer. Local Clang/LLDB required. Preview only; no Node.js needed.\n");
  execFileSync("tar", ["-czf", path.join(out, "c-observer-" + version + "-" + target + ".tar.gz"), "-C", dir, "."], { stdio: "inherit" });
}
await fs.writeFile(path.join(pkg, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
await fs.copyFile("docs/integration.md", path.join(pkg, "README.md"));
if (publishable) await fs.copyFile("LICENSE",path.join(pkg,"LICENSE"));
await fs.writeFile(path.join(pkg, "package.json"), JSON.stringify({
  name: "@annouimo/c-observer", version, description: "C Observer native binary distribution (preview)",
  repository: { type: "git", url: "git+https://github.com/Annouimo/c-observer.git" },
  ...(publishable ? { license: "SEE LICENSE IN LICENSE", publishConfig: { access: "public", tag: "next" } } : { private: true }),
  files: ["bin", "manifest.json", "README.md", "LICENSE"],
  exports: { "./manifest.json": "./manifest.json", "./package.json": "./package.json" }
}, null, 2) + "\n");
execFileSync("npm", ["pack", "--ignore-scripts", "--pack-destination", out], { cwd: pkg, stdio: "inherit" });
