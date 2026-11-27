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
export function memberLabel(m:Member):string { return m.index===undefined?"."+m.name:"["+m.index+"]"; }
export function columnLabel(c:Column):string { return c.name+(c.path??[]).map(memberLabel).join(""); }
export function comparable(v: Variable): boolean {
  return v.status === "observed" && !v.value_truncated &&
    (v.children?.length ? v.children_complete === true && v.children.every(comparable) : v.value !== undefined);
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

export type Shape={name:string;index?:number;kind:string;type:string;children:Shape[];arrayLength?:number};
export type Module={column:Column;shape:Shape};
const identity=(v:{name:string;index?:number})=>v.index===undefined?"n:"+v.name:"i:"+v.index;
function mergeShape(shape:Shape,v:Variable){
  if(v.status!=="observed")return;
  if(shape.kind!==v.kind||shape.type!==v.type){shape.kind="mixed";shape.type="多种类型";return;}
  if(v.array_length!==undefined)shape.arrayLength=Math.max(shape.arrayLength??0,v.array_length);
  const byKey=new Map(shape.children.map(c=>[identity(c),c]));
  for(const ch of v.children??[]){
    const key=identity(ch);
    let dest=byKey.get(key);
    if(!dest){dest=newShape(ch);shape.children.push(dest);byKey.set(key,dest);}
    mergeShape(dest,ch);
  }
}
function newShape(v:Variable):Shape{return {name:v.name,index:v.index,kind:v.kind,type:v.type,children:[],arrayLength:v.array_length};}
function finishShape(s:Shape){
  // Small declared arrays expose missing coordinates; no value is inferred.
  if(s.kind==="array"&&s.arrayLength!==undefined&&s.arrayLength<=32&&s.children.every(c=>c.index!==undefined)){
    const seen=new Set(s.children.map(c=>c.index));
    for(let i=0;i<s.arrayLength;i++)if(!seen.has(i))s.children.push({name:"["+i+"]",index:i,kind:"unknown",type:"",children:[]});
  }
  if(s.kind==="array")s.children.sort((a,b)=>(a.index??Infinity)-(b.index??Infinity));
  for(const child of s.children)finishShape(child);
}
export function modules(run:Run):Module[]{
  const map=new Map<string,Module>();
  for(const s of run.snapshots){
    const fn=s.frames[0]?.location.function??s.location.function;
    const pairs:[Column,Variable][]=[
      ...(s.frames[0]?.variables??[]).map(v=>[{name:v.name,fn},v] as [Column,Variable]),
      ...(s.globals??[]).map(v=>[{name:v.name,global:true},v] as [Column,Variable])
    ];
    for(const [c,v] of pairs){
      const key=columnKey(c);let m=map.get(key);
      if(!m){m={column:c,shape:newShape(v)};map.set(key,m);}
      mergeShape(m.shape,v);
    }
  }
  for(const m of map.values())finishShape(m.shape);
  // Scalars first; stable within category in order first encountered.
  return [...map.values()].sort((a,b)=>Number(!!a.shape.children.length||["array","struct","union"].includes(a.shape.kind))-Number(!!b.shape.children.length||["array","struct","union"].includes(b.shape.kind)));
}
export function member(c:Column,s:{name:string;index?:number}):Column{return {...c,path:[...(c.path??[]),{name:s.index===undefined?s.name:"["+s.index+"]",index:s.index}]};}
export function childValue(v:Variable|undefined,s:{name:string;index?:number}):Variable|undefined{
  if(!v||v.status!=="observed")return undefined;
  const matches=(v.children??[]).filter(c=>identity(c)===identity(s));
  return matches.length===1?matches[0]:undefined;
}
export function selectedValues(s:Snapshot,c:Column):Variable[]{
  // Unlike generic name fallback, root modules are always scoped to their
  // recorded local/global class. Array paths use actual indices, not labels.
  const fn=s.frames[0]?.location.function??s.location.function;
  let roots=c.global?(s.globals??[]):c.fn===fn?(s.frames[0]?.variables??[]):[];
  roots=roots.filter(v=>v.name===c.name);
  if(!c.path?.length)return roots;
  if(roots.length!==1)return [];
  let v:Variable|undefined=roots[0];
  for(const p of c.path)v=childValue(v,p);
  return v?[v]:[];
}
export function matchingPrevious(run:Run,s:Snapshot,c:Column):Variable|undefined{
  const p=run.snapshots[s.id-1];
  if(!p||!c.global&&(p.location.file!==s.location.file||p.location.function!==s.location.function||p.frames.length!==s.frames.length))return undefined;
  const before=selectedValues(p,c),now=selectedValues(s,c);
  return before.length===1&&now.length===1&&before[0].type===now[0].type&&before[0].kind===now[0].kind?before[0]:undefined;
}
export function observedChange(before:Variable|undefined,now:Variable|undefined):boolean{
  if(!before||!now||before.status!=="observed"||now.status!=="observed"||before.type!==now.type||before.kind!==now.kind)return false;
  if(now.children?.length){
    const seen=new Map<string,number>();for(const c of now.children)seen.set(identity(c),(seen.get(identity(c))??0)+1);
    return now.children.some(c=>seen.get(identity(c))===1&&observedChange(childValue(before,c),c));
  }
  return comparable(before)&&comparable(now)&&before.value!==now.value;
}
export function changeEvents(run:Run,c:Column):number[]{
  const out:number[]=[];
  for(const s of run.snapshots){
    const v=selectedValues(s,c);
    if(v.length===1&&observedChange(matchingPrevious(run,s,c),v[0]))out.push(s.id);
  }
  return out;
}
export function nextEvent(events:number[],step:number,direction:-1|1):number|undefined{
  return direction===1?events.find(id=>id>step):events.filter(id=>id<step).at(-1);
}
export function displayValue(v:Variable|undefined,root=false):string{
  return !v?(root?"—":"?"):v.status==="not_captured"?"?":v.status==="unavailable"?"无法读取":label(v);
}
export function sharedFields(shape:Shape):Shape[]{
  // Stable table columns for a matrix or array of structs; never invent values.
  const byKey=new Map<string,Shape>();
  for(const row of shape.children)for(const ch of row.children){
    const key=identity(ch);if(!byKey.has(key))byKey.set(key,ch);
  }
  const fields=[...byKey.values()];
  if(fields.every(f=>f.index!==undefined))fields.sort((a,b)=>a.index!-b.index!);
  return fields;
}
