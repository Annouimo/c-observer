import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";

// The shipped native binaries statically contain Go and Go dependencies.
// Preserve upstream license and notice texts, including module subdirectories.
const legalName = /^(?:licen[sc]es?|copying|copyright|notice|patents|unlicense)(?:$|[._-])/i;
async function legalFiles(root, recursive = true) {
  const results = [];
  async function visit(dir) {
    for (const entry of await fs.readdir(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name);
      if (entry.isDirectory() && recursive && entry.name !== ".git") await visit(file);
      else if (entry.isFile() && legalName.test(entry.name)) results.push(file);
    }
  }
  await visit(root);
  return results.sort();
}

export async function collectNotices(destination) {
  execFileSync("go", ["mod", "download", "all"], { stdio: "inherit" });
  const lines = execFileSync("go", ["list", "-m", "-f", "{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}", "all"], { encoding: "utf8" });
  const sections = [
    "# Third-party licenses and notices",
    "",
    "This distribution includes native Go binaries. The following upstream legal texts are retained from the Go toolchain and the Go module dependency graph, including modules that may not be linked in every target. External C compilers and debuggers are not bundled.",
    ""
  ];
  async function append(label, root, recursive) {
    if (!root) throw Error("Missing module directory for " + label);
    const files = await legalFiles(root, recursive);
    if (!files.some(file => /^(?:licen[sc]es?|copying|unlicense)(?:$|[._-])/i.test(path.basename(file))))
      throw Error("No upstream license found for " + label + "; review before redistribution");
    sections.push("## " + label, "");
    for (const file of files) {
      sections.push("--- " + path.relative(root, file).split(path.sep).join("/") + " ---", (await fs.readFile(file, "utf8")).trimEnd(), "");
    }
  }
  const goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
  const goversion = execFileSync("go", ["version"], { encoding: "utf8" }).trim();
  await append(goversion, goroot, false);
  for (const line of lines.split("\n").filter(Boolean)) {
    const [name, version, dir] = line.split("\t");
    await append(name + " " + version, dir, true);
  }
  await fs.writeFile(destination, sections.join("\n") + "\n");
}
