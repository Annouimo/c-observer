import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { collectNotices } from "./collect-notices.mjs";

const version = process.argv[2] || "0.1.0-dev.1";
if (!/^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(version)) throw Error("Invalid version");
const out = path.resolve(process.argv[3] || "dist-observer");
const publishable = process.argv[4] === "--publishable";
const stable = /^\d+\.\d+\.\d+$/.test(version);
if (publishable && !stable && !/^\d+\.\d+\.\d+-(?:alpha|beta|rc|dev)\.[0-9]+$/.test(version)) throw Error("Publisher requires a stable or explicit prerelease version");
const tag = stable ? "latest" : "next";
await fs.access("LICENSE");
const pkg = path.join(out, "npm");
const platforms = [
  ["darwin", "arm64", "darwin", "arm64"],
  ["darwin", "amd64", "darwin", "x64"],
  ["linux", "amd64", "linux", "x64"],
  ["windows", "amd64", "win32", "x64"]
];
await fs.mkdir(out, { recursive: true });
await fs.mkdir(pkg); // refuse to overwrite a prior build
await fs.copyFile("LICENSE",path.join(pkg,"LICENSE"));
await collectNotices(path.join(pkg,"THIRD_PARTY_NOTICES.txt"), platforms.map(([GOOS, GOARCH]) => ({ GOOS, GOARCH })));
const manifest = { schemaVersion: 1, version, sessionSchemaVersions: [1], platforms: {} };
for (const [goos, goarch, os, arch] of platforms) {
  const target = os + "-" + arch;
  const filename = goos === "windows" ? "c-observer.exe" : "c-observer";
  const relative = "bin/" + target + "/" + filename;
  const binary = path.join(pkg, relative);
  await fs.mkdir(path.dirname(binary), { recursive: true });
  execFileSync("go", ["build", "-trimpath", "-ldflags", "-s -w -X main.version=" + version, "-o", binary, "./cmd/c-observer"], {
    stdio: "inherit", env: { ...process.env, CGO_ENABLED: "0", GOOS: goos, GOARCH: goarch }
  });
  await fs.chmod(binary, 0o755);
  const sha256 = createHash("sha256").update(await fs.readFile(binary)).digest("hex");
  manifest.platforms[target] = { executable: relative, sha256, status: stable ? "stable" : "preview" };
  const dir = path.join(out, "standalone-" + target);
  await fs.mkdir(dir);
  await fs.copyFile(binary, path.join(dir, filename));
  await fs.chmod(path.join(dir, filename), 0o755);
  await fs.copyFile("docs/integration.md", path.join(dir, "integration.md"));
  await fs.copyFile(path.join(pkg,"LICENSE"),path.join(dir,"LICENSE"));
  await fs.copyFile(path.join(pkg,"THIRD_PARTY_NOTICES.txt"),path.join(dir,"THIRD_PARTY_NOTICES.txt"));
  await fs.writeFile(path.join(dir, "README.txt"), "C Observer " + version + "\nRun " + filename + ". Local GCC/GDB (Python-enabled) or Clang/LLDB required, depending on platform. " + (stable ? "Stable release." : "Preview only.") + " No Node.js needed.\n");
  if (goos === "windows") {
    execFileSync("zip", ["-qr", path.join(out, "c-observer-" + version + "-" + target + ".zip"), "."], { cwd: dir, stdio: "inherit" });
  } else {
    execFileSync("tar", ["-czf", path.join(out, "c-observer-" + version + "-" + target + ".tar.gz"), "-C", dir, "."], { stdio: "inherit" });
  }
}
await fs.writeFile(path.join(pkg, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
await fs.copyFile("docs/npm-readme.md", path.join(pkg, "README.md"));
await fs.writeFile(path.join(pkg, "package.json"), JSON.stringify({
  name: "@routilet/c-observer", version, description: "C Observer native binary distribution" + (stable ? "" : " (preview)"),
  repository: { type: "git", url: "git+https://github.com/Routilet/c-observer.git" },
  homepage: "https://github.com/Routilet/c-observer#readme",
  bugs: { url: "https://github.com/Routilet/c-observer/issues" },
  license: "MIT",
  ...(publishable ? { publishConfig: { access: "public", tag } } : { private: true }),
  files: ["bin", "manifest.json", "README.md", "LICENSE", "THIRD_PARTY_NOTICES.txt"],
  exports: { "./manifest.json": "./manifest.json", "./package.json": "./package.json" }
}, null, 2) + "\n");
execFileSync("npm", ["pack", "--ignore-scripts", "--pack-destination", out], { cwd: pkg, stdio: "inherit" });
