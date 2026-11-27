export type Variable = {
  name: string; type: string; kind: string; status: string; value?: string;
  index?: number; array_length?: number; children?: Variable[];
  children_complete?: boolean; value_truncated?: boolean; scope?: string; id?:string; identity_reliable?:boolean; declaration?:Location;
};
export type Location = { file: string; line: number; function?: string; reported_file?: string };
export type Frame = { id?:string; identity_reliable?:boolean; location: Location; variables: Variable[]; variables_complete?: boolean };
export type Snapshot = { id: number; location: Location; frames: Frame[]; globals?: Variable[]; globals_complete?: boolean };
export type Source = { path: string; text: string };
export type Run = { session: { sources: Source[]; backend: string; capabilities?:{globals?:boolean} }; snapshots: Snapshot[]; reason: string; message?: string; cleanup_error?: string; diagnostics?: {severity: string; message: string}[]; exit_code?: number; output: { stdout?: string; stderr?: string; stdout_truncated?: boolean; stderr_truncated?: boolean } };
export type Document = { run: Run; synthetic: boolean; stdin: string; expected: string | null; expectedBytes:Uint8Array|null; comparison: string; description?: string };
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
    expectedBytes:x.expected_stdout==null?null:synthetic?new TextEncoder().encode(read(x.expected_stdout)):bytes(x.expected_stdout),
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

export type Shape={name:string;index?:number;kind:string;type:string;children:Shape[];arrayLength?:number; width?:number; constant?:string; unstable?:boolean; important?:boolean};
export type Module={column:Column;shape:Shape};
const identity=(v:{name:string;index?:number})=>v.index===undefined?"n:"+v.name:"i:"+v.index;
function mergeShape(shape:Shape,v:Variable){
  shape.width=Math.max(shape.width??1,displayValue(v).length);
  if(v.status!=="observed"){shape.unstable=true;return;}
  if(!v.children?.length){
    if(!comparable(v))shape.unstable=true;
    else if(shape.constant===undefined)shape.constant=v.value;
    else if(shape.constant!==v.value)shape.unstable=true;
  }
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
      markImportant(m.shape,matchingPrevious(run,s,c),v);
    }
  }
  for(const m of map.values())finishShape(m.shape);
  // Scalars first; stable within category in order first encountered.
  return [...map.values()].sort((a,b)=>Number(!!a.shape.children.length||["array","struct","union"].includes(a.shape.kind))-Number(!!b.shape.children.length||["array","struct","union"].includes(b.shape.kind)));
}
export function member(c:Column,s:{name:string;index?:number}):Column{return {...c,path:[...(c.path??[]),{name:s.index===undefined?s.name:"["+s.index+"]",index:s.index}]};}
const childCache=new WeakMap<Variable,Map<string,Variable|undefined>>();
export function childValue(v:Variable|undefined,s:{name:string;index?:number}):Variable|undefined{
  if(!v||v.status!=="observed")return undefined;
  let map=childCache.get(v);
  if(!map){map=new Map();for(const ch of v.children??[]){const key=identity(ch);map.set(key,map.has(key)?undefined:ch);}childCache.set(v,map);}
  return map.get(identity(s));
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
function corresponding(a:Variable,b:Variable):boolean{
  return a.type===b.type&&a.kind===b.kind&&!(a.identity_reliable&&b.identity_reliable&&a.id&&b.id&&a.id!==b.id)&&!(a.scope&&b.scope&&a.scope!==b.scope)&&!(a.declaration&&b.declaration&&(a.declaration.file!==b.declaration.file||a.declaration.line!==b.declaration.line||a.declaration.function!==b.declaration.function));
}
export function matchingPrevious(run:Run,s:Snapshot,c:Column):Variable|undefined{
  const p=run.snapshots[s.id-1];
  if(!p||!c.global&&(p.location.file!==s.location.file||p.location.function!==s.location.function||p.frames.length!==s.frames.length))return undefined;
  const pf=p.frames[0],sf=s.frames[0];
  if(!c.global&&pf?.identity_reliable&&sf?.identity_reliable&&pf.id&&sf.id&&pf.id!==sf.id)return undefined;
  const root={...c,path:[]},before=selectedValues(p,root),now=selectedValues(s,root);
  if(before.length!==1||now.length!==1||!corresponding(before[0],now[0]))return undefined;
  let a:Variable|undefined=before[0],b:Variable|undefined=now[0];
  for(const part of c.path??[]){
    a=childValue(a,part);b=childValue(b,part);
    if(!a||!b||!corresponding(a,b))return undefined;
  }
  return a;
}
export function observedChange(before:Variable|undefined,now:Variable|undefined):boolean{
  if(!before||!now||before.status!=="observed"||now.status!=="observed"||!corresponding(before,now))return false;
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
  let lo=0,hi=events.length;
  while(lo<hi){const mid=(lo+hi)>>>1;if(direction===1?events[mid]<=step:events[mid]<step)lo=mid+1;else hi=mid;}
  return direction===1?events[lo]:events[lo-1];
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


// Shapes are derived once per record; never contain a displayed snapshot value.
function markImportant(shape:Shape,before:Variable|undefined,now:Variable){
  if(!before||!corresponding(before,now)||before.status!=="observed"||now.status!=="observed")return;
  if(now.children?.length){
    const map=new Map(shape.children.map(c=>[identity(c),c]));
    for(const ch of now.children){const dest=map.get(identity(ch));if(dest)markImportant(dest,childValue(before,ch),ch);}
    if(shape.children.some(c=>c.important))shape.important=true;
  }else if(observedChange(before,now))shape.important=true;
}
export function visibleModules(list:Module[],s:Snapshot|undefined):Module[]{
  if(!s)return [];
  const fn=s.frames[0]?.location.function??s.location.function;
  return list.filter(m=>m.column.global||m.column.fn===fn);
}
export function isCompound(s:Shape){return !!s.children.length||["array","struct","union"].includes(s.kind);}
export function moduleWidth(s:Shape):number{
  if(!isCompound(s))return Math.max(86,Math.min(450,(s.name.length+(s.width??1)+4)*8+12));
  const fields=s.kind==="array"?sharedFields(s):[];
  if(fields.length&&fields.every(c=>!isCompound(c)))return Math.max(180,Math.min(1100,42+fields.reduce((n,c)=>n+Math.max(45,Math.min(250,8*Math.max(c.name.length,c.width??1)+16)),0)));
  if(s.kind==="array"&&s.children.every(c=>!isCompound(c)))return Math.max(150,Math.min(1100,arraySegments(s).reduce((n,g)=>n+(g.length>1?126:Math.max(42,Math.min(240,8*Math.max(g[0].width??1,String(g[0].index??g[0].name).length)+16))),0)+14));
  return Math.max(210,Math.min(1000,s.children.reduce((n,c)=>n+moduleWidth(c)+6,0)+14));
}
export function arraySegments(s:Shape):Shape[][]{
  // Keep changing indices and immediate neighbours visible. Only long runs
  // of consecutive, historically constant, identically recorded leaves fold.
  const protectedIndices=new Set<number>();
  for(const ch of s.children)if(ch.important&&ch.index!==undefined)for(let d=-1;d<=1;d++)protectedIndices.add(ch.index+d);
  const foldable=(c:Shape)=>c.index!==undefined&&!protectedIndices.has(c.index)&&!c.important&&!c.unstable&&!isCompound(c)&&c.constant!==undefined;
  const out:Shape[][]=[];
  for(const c of s.children){
    const last=out.at(-1),p=last?.at(-1);
    if(p&&foldable(c)&&foldable(p)&&c.index===p.index!+1&&c.constant===p.constant&&c.type===p.type)last!.push(c);
    else out.push([c]);
  }
  return out.flatMap(g=>g.length>=4?[g]:g.map(c=>[c]));
}
export function foldValue(group:Shape[],value:Variable|undefined,previous:Variable|undefined):string|undefined{
  if(group.length<4)return undefined;
  const first=childValue(value,group[0]);
  if(!first||!comparable(first))return undefined;
  for(const c of group){
    const v=childValue(value,c);
    if(!v||!comparable(v)||v.value!==first.value||v.type!==first.type||v.kind!==first.kind||observedChange(childValue(previous,c),v))return undefined;
  }
  return first.value;
}
export function replayContext(run:Run,step:number){
  const after=run.snapshots[step]?.location,before=run.snapshots[step-1]?.location;
  return {origin:before??after,after,before};
}
export function markerPath(events:number[],total:number):string{
  if(total<2)return "";
  // At most 1001 rendered bins; exact events retained for navigation.
  return [...new Set(events.map(id=>Math.round(id/(total-1)*1000)))].map(x=>"M"+x+" 0V12").join(" ");
}
const sameBytes=(a:Uint8Array,b:Uint8Array)=>a.length===b.length&&a.every((v,i)=>v===b[i]);
export function sampleResult(doc:Document):string{
  const r=doc.run;
  if(r.reason!=="exited"||r.exit_code!=null&&r.exit_code!==0)return "运行未正常结束";
  if(r.output.stdout_truncated)return "输出不完整";
  if(doc.expectedBytes===null)return "未提供预期";
  const actual=bytes(r.output.stdout??""),expected=doc.expectedBytes;
  let equal=sameBytes(actual,expected);
  if(doc.comparison){
    try{
      const decoder=new TextDecoder("utf-8",{fatal:true,ignoreBOM:true});
      const normalize=(a:Uint8Array)=>{
        const t=decoder.decode(a).replace(/\r\n/g,"\n");
        if(doc.comparison==="trim-line-end")return t.split("\n").map(s=>s.replace(/[ \t]+$/g,"")).join("\n").replace(/\n+$/g,"");
        if(doc.comparison==="tokens")return t.split(/\s+/u).filter(Boolean).join("\n");
        return t;
      };
      equal=normalize(expected)===normalize(actual);
    }catch{/* invalid UTF-8: raw bytes */}
  }
  return equal?"输出一致":"输出不一致";
}
export function outputRows(doc:Document):{expected?:string;actual?:string;equal:boolean}[]{
  const split=(a:Uint8Array)=>{
    const result:Uint8Array[]=[];let start=0;
    for(let i=0;i<a.length;i++)if(a[i]===10){result.push(a.slice(start,i+1));start=i+1;}
    if(start<a.length)result.push(a.slice(start));return result;
  };
  const show=(a:Uint8Array)=>{
    let text="";
    // Invalid UTF-8 is escaped bytewise; equality always uses raw bytes.
    try{text=new TextDecoder("utf-8",{fatal:true,ignoreBOM:true}).decode(a);}
    catch{return Array.from(a,b=>b===10?"↵":b===13?"\\r":b===9?"\\t":b===32?"·":b>=33&&b<=126?String.fromCharCode(b):"\\x"+b.toString(16).padStart(2,"0")).join("");}
    return text.replace(/[\x00-\x20\x7f]/g,c=>c==="\n"?"↵":c==="\r"?"\\r":c==="\t"?"\\t":c===" "?"·":"\\x"+c.charCodeAt(0).toString(16).padStart(2,"0"));
  };
  const a=split(doc.expectedBytes??new Uint8Array()),b=split(bytes(doc.run.output.stdout??""));
  return Array.from({length:Math.max(a.length,b.length)},(_,i)=>({expected:a[i]?show(a[i]):undefined,actual:b[i]?show(b[i]):undefined,equal:!!a[i]&&!!b[i]&&sameBytes(a[i],b[i])}));
}
export type Token={text:string;kind:string};
export function sourceTokens(text:string):Token[][]{
  const lines:Token[][]=[[]];
  const re=/\/\*[\s\S]*?(?:\*\/|$)|\/\/[^\r\n]*|"(?:\\[\s\S]|[^"\\\r\n])*"|'(?:\\[\s\S]|[^'\\\r\n])*'|\b(?:if|else|for|while|do|switch|case|break|continue|return|int|long|short|float|double|char|void|struct|typedef|const|static|unsigned|signed|sizeof|enum|union|default)\b|\b\d+(?:\.\d+)?\b|[A-Za-z_]\w*|\r?\n|[\s\S]/g;
  for(const m of text.matchAll(re)){
    const t=m[0],kind=t.startsWith("//")||t.startsWith("/*")?"comment":t.startsWith('"')||t.startsWith("'")?"literal":/^(if|else|for|while|do|switch|case|break|continue|return|int|long|short|float|double|char|void|struct|typedef|const|static|unsigned|signed|sizeof|enum|union|default)$/.test(t)?"keyword":/^\d/.test(t)?"number":"plain";
    const parts=t.split(/\r?\n/);
    for(let i=0;i<parts.length;i++){if(i)lines.push([]);if(parts[i]){const l=lines.at(-1)!;if(l.at(-1)?.kind===kind)l.at(-1)!.text+=parts[i];else l.push({text:parts[i],kind});}}
  }
  return lines;
}
