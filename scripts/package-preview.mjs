import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { collectNotices } from "./collect-notices.mjs";

const version = process.argv[2] || "0.1.0-dev.1";
if (!/^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(version)) throw Error("Invalid version");
const out = path.resolve(process.argv[3] || "dist-observer");
const publishable = process.argv[4] === "--publishable";
if (publishable && !/^\d+\.\d+\.\d+-(?:alpha|beta|rc|dev)\.[0-9]+$/.test(version)) throw Error("Preview publisher only accepts explicit prerelease versions");
await fs.access("LICENSE");
const pkg = path.join(out, "npm");
const platforms = [
  ["darwin", "arm64", "darwin", "arm64"],
  ["darwin", "amd64", "darwin", "x64"],
  ["linux", "amd64", "linux", "x64"]
];
await fs.mkdir(out, { recursive: true });
await fs.mkdir(pkg); // refuse to overwrite a prior build
await fs.copyFile("LICENSE",path.join(pkg,"LICENSE"));
await collectNotices(path.join(pkg,"THIRD_PARTY_NOTICES.txt"), platforms.map(([GOOS, GOARCH]) => ({ GOOS, GOARCH })));
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
  await fs.copyFile(path.join(pkg,"LICENSE"),path.join(dir,"LICENSE"));
  await fs.copyFile(path.join(pkg,"THIRD_PARTY_NOTICES.txt"),path.join(dir,"THIRD_PARTY_NOTICES.txt"));
  await fs.writeFile(path.join(dir, "README.txt"), "C Observer " + version + "\nRun ./c-observer. Local Clang/LLDB required. Preview only; no Node.js needed.\n");
  execFileSync("tar", ["-czf", path.join(out, "c-observer-" + version + "-" + target + ".tar.gz"), "-C", dir, "."], { stdio: "inherit" });
}
await fs.writeFile(path.join(pkg, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
await fs.copyFile("docs/npm-readme.md", path.join(pkg, "README.md"));
await fs.writeFile(path.join(pkg, "package.json"), JSON.stringify({
  name: "@annouimo/c-observer", version, description: "C Observer native binary distribution (preview)",
  repository: { type: "git", url: "git+https://github.com/Annouimo/c-observer.git" },
  license: "MIT",
  ...(publishable ? { publishConfig: { access: "public", tag: "next" } } : { private: true }),
  files: ["bin", "manifest.json", "README.md", "LICENSE", "THIRD_PARTY_NOTICES.txt"],
  exports: { "./manifest.json": "./manifest.json", "./package.json": "./package.json" }
}, null, 2) + "\n");
execFileSync("npm", ["pack", "--ignore-scripts", "--pack-destination", out], { cwd: pkg, stdio: "inherit" });
