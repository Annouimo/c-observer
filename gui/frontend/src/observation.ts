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
export type Member = {name:string; index?:number};
export type Column = { name: string; fn?: string; global?: boolean; path?: Member[] };
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
  // Go's nil slices serialize as null, notably on compile/startup failure.
  const r = x.run;
  if (r?.session?.sources === null) r.session.sources = [];
  if (r?.snapshots === null) r.snapshots = [];
  if (Array.isArray(r?.snapshots)) for (const s of r.snapshots) {
    if (s?.frames === null) s.frames = [];
    if (Array.isArray(s?.frames)) for (const f of s.frames) {
      if (f?.variables === null) f.variables = [];
    }
    if (s?.globals === null) s.globals = [];
  }
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
export const columnKey = (c: Column) => JSON.stringify([c.name, c.fn ?? "", !!c.global, c.path ?? []]);
function rootValues(s: Snapshot, c: Column): Variable[] {
  if (c.global) return (s.globals ?? []).filter(v => v.name === c.name);
  const frame = s.frames[0];
  const fn = frame?.location.function ?? s.location.function;
  const locals = !c.fn || c.fn === fn ? (frame?.variables ?? []).filter(v => v.name === c.name) : [];
  return c.fn ? locals : [...locals, ...(s.globals ?? []).filter(v => v.name === c.name)];
}
export function values(s: Snapshot,c: Column): Variable[] {
  let found=rootValues(s,c);
  for(const part of c.path??[]) found=found.flatMap(v=>v.status==="observed"?(v.children??[]).filter(ch=>ch.name===part.name&&ch.index===part.index):[]);
  return found;
}
export function memberLabel(m:Member):string { return m.index===undefined?"."+m.name:"["+m.index+"]"; }
export function columnLabel(c:Column):string { return c.name+(c.path??[]).map(memberLabel).join(""); }
export function memberColumns(v:Variable,c:Column):Column[] {
  return (v.children??[]).map(ch=>({...c,path:[...(c.path??[]),{name:ch.name,index:ch.index}]}));
}
export type Difference={path:string;before:string;after:string};
export function differences(previous:Variable|undefined,value:Variable,path="",limit=8):Difference[] {
  if(limit<=0||!previous||previous.status!=="observed"||value.status!=="observed"||previous.type!==value.type||previous.kind!==value.kind) return [];
  if(value.children?.length) {
    const out:Difference[]=[];
    for(const ch of value.children) {
      out.push(...differences(childBefore(previous,ch),ch,path+memberLabel(ch),limit-out.length));
      if(out.length>=limit) break;
    }
    return out;
  }
  return valueChanged(previous,value)?[{path,before:label(previous),after:label(value)}]:[];
}
export function previewValues(v:Variable,limit=6):{path:string;value:string}[] {
  const out:{path:string;value:string}[]=[];
  function visit(x:Variable,path:string) {
    if(out.length>=limit) return;
    if(x.status==="observed"&&x.children?.length) for(const ch of x.children) {visit(ch,path+memberLabel(ch));if(out.length>=limit) break;}
    else out.push({path,value:label(x)});
  }
  visit(v,"");
  return out;
}
export function comparable(v: Variable): boolean {
  return v.status === "observed" && !v.value_truncated &&
    (v.children?.length ? v.children_complete === true && v.children.every(comparable) : v.value !== undefined);
}
// Only compare unique, same-name/same-type children; partial arrays can
// still provide real evidence of differences at recorded indices.
export function childBefore(previous: Variable | undefined, child: Variable): Variable | undefined {
  if (!previous || previous.status !== "observed") return undefined;
  const candidates = (previous.children ?? []).filter(c => c.name === child.name && c.index === child.index && c.type === child.type && c.kind === child.kind);
  return candidates.length === 1 ? candidates[0] : undefined;
}
export function valueChanged(previous: Variable | undefined, value: Variable): boolean {
  if (!previous || previous.status !== "observed" || value.status !== "observed" || previous.type !== value.type || previous.kind !== value.kind) return false;
  if (value.children?.length) return value.children.some(c => valueChanged(childBefore(previous,c),c));
  return comparable(previous) && comparable(value) && previous.value !== value.value;
}
export function previousValue(run: Run, s: Snapshot, c: Column): Variable | undefined {
  if (!s.id) return undefined;
  const p = run.snapshots[s.id-1];
  if (!p || (!c.global && (p.location.function !== s.location.function || p.frames.length !== s.frames.length))) return undefined;
  const a = values(p,c), b = values(s,c);
  return a.length === 1 && b.length === 1 && a[0].type === b[0].type ? a[0] : undefined;
}
export function changed(run: Run, s: Snapshot, c: Column): boolean {
  const v=values(s,c);
  return v.length===1 && valueChanged(previousValue(run,s,c),v[0]);
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
export type TraceNode = { kind:"observation"; snapshot: Snapshot } | { kind:"group"; key:string; label:string; children:TraceNode[]; anchor:Location; groupType:"loop"|"round"|"call" };
export function flatten(nodes: TraceNode[]): Snapshot[] {
  return nodes.flatMap(n => n.kind === "observation" ? [n.snapshot] : flatten(n.children));
}
type LoopRange={file:string;start:number;end:number};
export function loopRanges(source:Source):LoopRange[] {
  const ts=tokenize(source.text).flatMap((line,i)=>line.filter(t=>t.kind!=="comment"&&t.kind!=="literal"&&t.text.trim()).map(t=>({...t,line:i+1})));
  const out:LoopRange[]=[];
  function close(i:number,open:string,close:string):number {
    let depth=0;
    for(let j=i;j<ts.length;j++){if(ts[j].text===open)depth++;else if(ts[j].text===close&&--depth===0)return j;}
    return -1;
  }
  for(let i=0;i<ts.length;i++) {
    if(ts[i].kind!=="keyword"||!["for","while","do"].includes(ts[i].text))continue;
    let j=i+1;
    if(ts[i].text!=="do"){if(ts[j]?.text!=="(")continue;j=close(j,"(",")")+1;if(j<=0)continue;}
    let end=-1;
    if(ts[j]?.text==="{")end=close(j,"{","}");
    else if(ts[j]&&![";","for","while","do","if","switch","else"].includes(ts[j].text)){
      // Common unbraced statement only. Ambiguous nested controls stay raw.
      let depth=0;
      for(let k=j;k<ts.length;k++){const t=ts[k].text;if(t==="("||t==="[")depth++;if(t===")"||t==="]")depth--;if(t==="{"||t==="}")break;if(t===";"&&depth===0){end=k;break;}}
    }
    if(end>=j&&ts[end].line>ts[i].line)out.push({file:source.path,start:ts[i].line,end:ts[end].line});
  }
  return out.sort((a,b)=>a.start-b.start||b.end-a.end);
}
const sameContext=(a:Snapshot,b:Snapshot)=>a.location.file===b.location.file&&a.location.function===b.location.function&&a.frames.length===b.frames.length;
const observation=(s:Snapshot):TraceNode=>({kind:"observation",snapshot:s});
// Presentation hints only. Every original snapshot appears exactly once,
// and flatten(groupTrace(run)) retains the original time order.
export function groupTrace(run:Run):TraceNode[] {
  const ranges=run.session.sources.flatMap(loopRanges);
  function build(ss:Snapshot[],level:number,excluded:LoopRange[]=[]):TraceNode[]{
    if(level>=32)return ss.map(observation);
    const out:TraceNode[]=[];
    for(let i=0;i<ss.length;){
      const first=ss[i],depth=first.frames.length;
      const candidates=ranges.filter(r=>!excluded.includes(r)&&r.file===first.location.file&&first.location.line>=r.start&&first.location.line<=r.end);
      let grouped=false;
      for(const r of candidates){
        let j=i+1;
        while(j<ss.length){
          const t=ss[j];
          if(depth>0&&t.frames.length>depth){j++;continue;}
          if(!sameContext(first,t)||t.location.line<r.start||t.location.line>r.end)break;
          j++;
        }
        const startsAtHeader=first.location.line===r.start;
        const anchors:number[]=[];
        for(let k=i;k<j;k++)if(sameContext(first,ss[k])&&ss[k].location.line===r.start)anchors.push(k);
        const parts:Snapshot[][]=[];
        let tail:Snapshot[]=[];
        if(startsAtHeader){
          for(let a=0;a+1<anchors.length;a++)if(anchors[a+1]-anchors[a]>1)parts.push(ss.slice(anchors[a],anchors[a+1]));else {parts.length=0;break;}
          if(parts.length>=2)tail=ss.slice(anchors.at(-1)!,j);
        }else{
          let k=i;
          for(const a of anchors){if(a>k){parts.push(ss.slice(k,a+1));k=a+1;}else{parts.length=0;break;}}
          if(parts.length>=2)tail=ss.slice(k,j);
        }
        if(parts.length<2)continue;
        const anchor={...first.location,line:r.start};
        const rounds:TraceNode[]=parts.map((p,n)=>({kind:"group",groupType:"round",key:"round-"+p[0].id+"-"+r.start,label:"第 "+(n+1)+" 组观察",anchor,children:build(p,level+1,[...excluded,r])}));
        out.push({kind:"group",groupType:"loop",key:"loop-"+first.id+"-"+r.start,label:rounds.length+" 组重复观察",anchor,children:rounds});
        out.push(...build(tail,level+1,[...excluded,r]));
        i=j;grouped=true;break;
      }
      if(grouped)continue;
      if(depth>0&&i+1<ss.length&&ss[i+1].frames.length>depth){
        let j=i+1;while(j<ss.length&&ss[j].frames.length>depth)j++;
        out.push(observation(first),{kind:"group",groupType:"call",key:"call-"+ss[i+1].id,label:(ss[i+1].location.function??"未知函数")+" · 调用观察",anchor:first.location,children:build(ss.slice(i+1,j),level+1)});
        i=j;continue;
      }
      out.push(observation(first));i++;
    }
    return out;
  }
  return build(run.snapshots,0);
}
export type SourceRow={line:number;tokens:Token[];nodes:TraceNode[];count:number;fn?:string};
// Static source is the document skeleton. Dynamic nodes are attached, never
// used to fabricate states for lines lacking an observation. Unknown locations
// remain accessible in the chronological view / unmapped section.
export function sourceDocument(run:Run,path:string,tree=groupTrace(run)):{rows:SourceRow[];unmapped:TraceNode[]} {
  const source=run.session.sources.find(s=>s.path===path);
  const rows:SourceRow[]=source?tokenize(source.text).map((tokens,i)=>({line:i+1,tokens,nodes:[],count:0})):[];
  for(const s of run.snapshots)if(s.location.file===path&&rows[s.location.line-1]){
    const row=rows[s.location.line-1];row.count++;row.fn??=s.location.function;
  }
  const unmapped:TraceNode[]=[];
  for(const node of tree){
    const loc=node.kind==="observation"?node.snapshot.location:node.anchor;
    if(loc.file===path&&rows[loc.line-1])rows[loc.line-1].nodes.push(node);
    else if(!run.session.sources.some(s=>s.path===loc.file&&loc.line>0&&loc.line<=s.text.split(/\r?\n/).length))unmapped.push(node);
  }
  return {rows,unmapped};
}
export function groupDifferences(run:Run,n:TraceNode,c:Column,limit=3):Difference[]{
  const out:Difference[]=[];
  if(n.kind!=="group")return out;
  for(const s of flatten(n.children)){
    const v=values(s,c);
    if(v.length===1)out.push(...differences(previousValue(run,s,c),v[0],"",limit-out.length));
    if(out.length>=limit)break;
  }
  return out;
}
