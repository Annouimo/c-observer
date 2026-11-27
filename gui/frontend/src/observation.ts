export type Variable = {
  name: string; type: string; kind: string; status: string; value?: string;
  index?: number; array_length?: number; children?: Variable[];
  children_complete?: boolean; value_truncated?: boolean; scope?: string;
};
export type Location = { file: string; line: number; function?: string; reported_file?: string };
export type Frame = { location: Location; variables: Variable[]; variables_complete?: boolean };
export type Snapshot = { id: number; location: Location; frames: Frame[]; globals?: Variable[]; globals_complete?: boolean };
export type Source = { path: string; text: string };
export type Run = { session: { sources: Source[]; backend: string }; snapshots: Snapshot[]; reason: string; message?: string; cleanup_error?: string; diagnostics?: {severity: string; message: string}[]; exit_code?: number; output: { stdout?: string; stderr?: string; stdout_truncated?: boolean; stderr_truncated?: boolean } };
export type Document = { run: Run; synthetic: boolean; stdin: string; expected: string | null; comparison: string; description?: string };
export type Column = { name: string; fn?: string; global?: boolean };
export const MAX_BYTES = 128 * 1024 * 1024;
const bytes = (s: string) => Uint8Array.from(atob(s), c => c.charCodeAt(0));
const decode = (s: string) => new TextDecoder().decode(bytes(s));
function validVariable(v: any, depth = 0): boolean {
  return depth <= 32 && v && typeof v.name === "string" && typeof v.type === "string" &&
    typeof v.kind === "string" && ["observed", "not_captured", "unavailable"].includes(v.status) &&
    (v.value === undefined || typeof v.value === "string") &&
    (v.index === undefined || Number.isSafeInteger(v.index) && v.index >= 0) &&
    (v.array_length === undefined || Number.isSafeInteger(v.array_length) && v.array_length >= 0) &&
    (v.children === undefined || Array.isArray(v.children) && v.children.every((c: any) => validVariable(c, depth + 1)));
}
function validLocation(l: any): boolean {
  return l && typeof l.file === "string" && Number.isSafeInteger(l.line) && l.line >= 0 &&
    (l.function === undefined || typeof l.function === "string");
}
export function parseDocument(x: any): Document {
  const synthetic = x?.synthetic === true && x?.run?.session?.backend === "synthetic";
  if (!synthetic && (x?.format !== "c-observer-record" || x.schema_version !== 1 || x?.run?.session?.backend === "synthetic")) throw Error("不支持的观察记录格式");
  const r = x.run;
  if (!r || !r.session || typeof r.session.backend !== "string" ||
    !Array.isArray(r.session.sources) || !r.session.sources.every((s: any) => s && typeof s.path === "string" && typeof s.text === "string") ||
    !Array.isArray(r.snapshots) || typeof r.reason !== "string" || !r.output ||
    !r.snapshots.every((s: any, i: number) => s && s.id === i && validLocation(s.location) &&
      Array.isArray(s.frames) && s.frames.every((f: any) => f && validLocation(f.location) && Array.isArray(f.variables) && f.variables.every((v: any) => validVariable(v))) &&
      (s.globals === undefined || Array.isArray(s.globals) && s.globals.every((v: any) => validVariable(v))))) throw Error("观察记录数据不完整或损坏");
  const read = (s: any) => {
    if (s == null) return "";
    if (typeof s !== "string") throw Error("无效输入输出字段");
    return synthetic ? s : decode(s);
  };
  return { run: r, synthetic, stdin: read(x.stdin),
    expected: x.expected_stdout == null ? null : read(x.expected_stdout),
    comparison: typeof x.comparison === "string" ? x.comparison : "exact", description: x.description };
}
export function outputText(s?: string) { return decode(s ?? ""); }
export function sourceLine(run: Run, loc: Location): string {
  return run.session.sources.find(s => s.path === loc.file)?.text.split(/\r?\n/)[loc.line - 1] ?? "";
}
export const columnKey = (c: Column) => JSON.stringify([c.name, c.fn ?? "", !!c.global]);
export function values(s: Snapshot, c: Column): Variable[] {
  if (c.global) return (s.globals ?? []).filter(v => v.name === c.name);
  const frame = s.frames[0];
  const fn = frame?.location.function ?? s.location.function;
  const locals = !c.fn || c.fn === fn ? (frame?.variables ?? []).filter(v => v.name === c.name) : [];
  return c.fn ? locals : [...locals, ...(s.globals ?? []).filter(v => v.name === c.name)];
}
export function comparable(v: Variable): boolean {
  return v.status === "observed" && !v.value_truncated &&
    (v.children?.length ? v.children_complete === true && v.children.every(comparable) : v.value !== undefined);
}
function signature(v: Variable): string {
  return JSON.stringify([v.type,v.kind,v.value,v.index,v.array_length,v.children?.map(c => [c.name,signature(c)])]);
}
export function changed(run: Run, s: Snapshot, c: Column): boolean {
  if (!s.id) return false;
  const p = run.snapshots[s.id-1];
  if (!p || (!c.global && (p.location.function !== s.location.function || p.frames.length !== s.frames.length))) return false;
  const a = values(p,c), b = values(s,c);
  return a.length === 1 && b.length === 1 && a[0].type === b[0].type && comparable(a[0]) && comparable(b[0]) && signature(a[0]) !== signature(b[0]);
}
export function label(v: Variable): string {
  if (v.status === "not_captured") return "未采集";
  if (v.status === "unavailable") return "无法读取";
  if (v.children?.length) {
    const n = v.children.length;
    const summary = v.kind === "array" ? (v.array_length === undefined ? n + " 项已记录" : n + "/" + v.array_length + " 项") : n + " 个成员";
    return summary + (v.children_complete === false ? " · 部分" : "");
  }
  return (v.value ?? "无已记录值") + (v.value_truncated ? " · 截断" : "");
}
export type Token = { text: string; kind: "identifier" | "keyword" | "literal" | "comment" | "other" };
const keywords = new Set("auto break case char const continue default do double else enum extern float for goto if inline int long register restrict return short signed sizeof static struct switch typedef union unsigned void volatile while _Bool _Complex _Imaginary".split(" "));
export function tokenize(text: string): Token[][] {
  const result: Token[][] = [[]];
  const re = /\/\*[\s\S]*?(?:\*\/|$)|\/\/[^\r\n]*|"(?:\\[\s\S]|[^"\\])*"|'(?:\\[\s\S]|[^'\\])*'|[A-Za-z_]\w*|\r?\n|[^\r\n]/g;
  for (const m of text.matchAll(re)) {
    const t = m[0];
    const kind = t.startsWith("//") || t.startsWith("/*") ? "comment" : t.startsWith('"') || t.startsWith("'") ? "literal" : keywords.has(t) ? "keyword" : /^[A-Za-z_]\w*$/.test(t) ? "identifier" : "other";
    const lines = t.split(/\r?\n/);
    lines.forEach((part,i) => { if(i) result.push([]); if(part) result.at(-1)!.push({text:part,kind}); });
  }
  return result;
}
export type TraceNode = { kind:"observation"; snapshot: Snapshot } | { kind:"group"; key:string; label:string; children:TraceNode[] };
export function flatten(nodes: TraceNode[]): Snapshot[] {
  return nodes.flatMap(n => n.kind === "observation" ? [n.snapshot] : flatten(n.children));
}
// Conservative grouping only: repeated observed loop headers within the same
// file, function and depth, with consecutive nonempty body segments. No
// fabricated iterations, condition outcomes, return values or observations.
export function groupTrace(run: Run): TraceNode[] {
  const headers = new Set<string>();
  for (const source of run.session.sources) tokenize(source.text).forEach((tokens,i) => {
    if(tokens.some(t => t.kind === "keyword" && ["for","while","do"].includes(t.text))) headers.add(JSON.stringify([source.path,i+1]));
  });
  const key = (s: Snapshot) => JSON.stringify([s.location.file,s.location.line]);
  const same = (a: Snapshot,b: Snapshot) => a.location.file === b.location.file && a.location.function === b.location.function && a.frames.length === b.frames.length;
  const obs = (s: Snapshot): TraceNode => ({kind:"observation",snapshot:s});
  // At most 64 nested groups; raw observations are always preserved.
  function build(ss: Snapshot[], level: number): TraceNode[] {
    if(level >= 64) return ss.map(obs);
    const out: TraceNode[] = [];
    for(let i=0;i<ss.length;) {
      const start=ss[i], depth=start.frames.length;
      if(depth>0 && i+1<ss.length && ss[i+1].frames.length>depth) {
        let j=i+1; while(j<ss.length && ss[j].frames.length>depth) j++;
        out.push(obs(start), {kind:"group",key:"call-"+ss[i+1].id,label:"较深调用观察 · "+(ss[i+1].location.function??"未知函数"),children:build(ss.slice(i+1,j),level+1)});
        i=j; continue;
      }
      if(headers.has(key(start))) {
        let j=i+1;
        const anchors=[i];
        // Bound by same-context backward jump or a different source/function.
        // Only group exact repeated header-to-header segments with at least one
        // body observation; the last header remains a separate raw observation.
        for(;j<ss.length;j++) {
          if(!same(start,ss[j]) || ss[j].location.line<start.location.line) break;
          if(key(start)===key(ss[j])) anchors.push(j);
          // If another loop header occurs, the conservative first pass stops.
          else if(headers.has(key(ss[j]))) break;
        }
        let end=1;
        while(end<anchors.length && anchors[end]-anchors[end-1]>1) end++;
        if(end>=3) {
          const children: TraceNode[]=[];
          for(let a=0;a<end-1;a++) {
            const slice=ss.slice(anchors[a],anchors[a+1]);
            children.push({kind:"group",key:"repeat-"+slice[0].id,label:"片段 "+(a+1),children:slice.map(obs)});
          }
          out.push({kind:"group",key:"loop-"+start.id,label:"重复入口 · "+start.location.line+" 行 · "+children.length+" 组",children});
          i=anchors[end-1]; continue;
        }
      }
      out.push(obs(start)); i++;
    }
    return out;
  }
  return build(run.snapshots,0);
}
