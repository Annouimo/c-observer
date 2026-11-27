<script lang="ts">
  import example from "../../../testdata/snapshots/wrong-index.json";
  import DataValue from "./DataValue.svelte";
  import { parseDocument, tokenize, groupTrace, values, changed, previousValue, columnKey, outputText, MAX_BYTES, type Column, type Location, type Snapshot, type TraceNode, type Token } from "./observation";

  let doc = $state(parseDocument(example));
  let mode = $state<"trace"|"source"|"io">("trace");
  let columns = $state<Column[]>([{name:"i",fn:"main"},{name:"index",fn:"main"},{name:"a",fn:"main"}]);
  let filter = $state<{file:string;line:number}|null>(null);
  let open = $state<string[]>([]);
  let detail = $state<number|null>(null);
  let grouped = $state(true);
  let error = $state("");
  let selectedPath = $state(example.run.session.sources[0]?.path ?? "");
  let lines = $derived(tokenize(doc.run.session.sources.find(s=>s.path===selectedPath)?.text??""));
  let sourceTokens = $derived(new Map(doc.run.session.sources.map(s=>[s.path,tokenize(s.text)])));
  let tree = $derived(groupTrace(doc.run));
  let names = $derived([...new Map<string,Column>(doc.run.snapshots.flatMap(s=>[
    ...(s.frames[0]?.variables??[]).map(v=>({name:v.name,fn:s.frames[0]?.location.function??s.location.function})),
    ...(s.globals??[]).map(v=>({name:v.name,global:true}))
  ]).map(c=>[columnKey(c),c] as [string,Column])).values()]);
  let selectedColumn = $state("");
  let matching = $derived(filter ? doc.run.snapshots.filter(s=>s.location.file===filter!.file && s.location.line===filter!.line) : doc.run.snapshots);
  let nodes = $derived(grouped&&!filter ? tree : matching.map(s=>({kind:"observation",snapshot:s} as TraceNode)));
  let actual = $derived(outputText(doc.run.output.stdout));
  let stderr = $derived(outputText(doc.run.output.stderr));
  let rawEqual = $derived(doc.expected !== null && !doc.run.output.stdout_truncated && actual === doc.expected);
  async function load(e: Event) {
    const input=e.currentTarget as HTMLInputElement;
    const file=input.files?.[0]; if(!file) return;
    try {
      if(file.size>MAX_BYTES) throw Error("文件超过 128 MiB");
      const next=parseDocument(JSON.parse(await file.text()));
      outputText(next.run.output.stdout); outputText(next.run.output.stderr);
      doc=next; columns=[]; open=[]; detail=null; filter=null; selectedColumn="";
      selectedPath=next.run.session.sources[0]?.path??""; mode="trace";error="";
    } catch(e) { error=e instanceof Error?e.message:String(e); }
    input.value="";
  }
  function toggle(c: Column) {
    const k=columnKey(c);
    columns=columns.some(x=>columnKey(x)===k)?columns.filter(x=>columnKey(x)!==k):[...columns,c];
  }
  function choose(name: string, fn?: string) {
    const c=names.find(x=>x.name===name && x.fn===fn && !x.global)??names.find(x=>x.name===name && x.global);
    if(c) toggle(c);
    else if(names.some(x=>x.name===name)) toggle({name,fn});
  }
  function selectable(name:string) { return names.some(c=>c.name===name); }
  function selectLine(loc: Location) { filter={file:loc.file,line:loc.line};mode="trace";detail=null; }
  function expand(key:string) { open=open.includes(key)?open.filter(k=>k!==key):[...open,key]; }
  function lineCount(line:number) { return doc.run.snapshots.filter(s=>s.location.file===selectedPath&&s.location.line===line).length; }
  function contextAt(line:number) { return doc.run.snapshots.find(s=>s.location.file===selectedPath&&s.location.line===line)?.location.function; }
  function tokensAt(s: Snapshot) { return sourceTokens.get(s.location.file)?.[s.location.line-1]??[]; }
</script>

{#snippet code(tokens: Token[], fn: string | undefined)}
  <span class="code">{#each tokens as token}{#if token.kind==="identifier"&&selectable(token.text)}
    <button class="token" class:selected={columns.some(c=>c.name===token.text&&(!c.fn||c.fn===fn))} onclick={()=>choose(token.text,fn)} title="添加或移除观察列">{token.text}</button>
  {:else}<span class={token.kind}>{token.text}</span>{/if}{/each}</span>
{/snippet}
{#snippet snapshot(s:Snapshot,indent:number)}
  <tr>
    <td class="seq">{s.id+1}</td>
    <td><button class="line" onclick={()=>selectLine(s.location)} disabled={!s.location.line||!s.location.file}>{s.location.line||"—"}</button></td>
    <td class="source-cell"><div style:padding-left={indent*16+"px"}>{#if tokensAt(s).length}{@render code(tokensAt(s),s.location.function)}{:else}<span class="muted">{s.location.reported_file??"无可映射源码"}</span>{/if}
      <small>{s.location.function??""} {s.location.file.split(/[\\/]/).at(-1)??""}</small>
      <button class="inline-control" onclick={()=>detail=detail===s.id?null:s.id}>{detail===s.id?"收起状态":"全部状态"}</button></div></td>
    {#each columns as c (columnKey(c))}
      <td class:changed={changed(doc.run,s,c)}>{#each values(s,c) as v}<DataValue value={v} previous={previousValue(doc.run,s,c)}/>{:else}<span class="muted" title="当前记录无对应值">—</span>{/each}</td>
    {/each}
  </tr>
  {#if detail===s.id}
    <tr><td colspan={3+columns.length}><div class="detail">
      <strong>观察 #{s.id+1} · 暂停状态</strong>
      {#each [{name:"当前帧",vars:s.frames[0]?.variables??[]},{name:"全局/静态",vars:s.globals??[]}] as group}
        <div><small>{group.name}</small><div class="state-grid">{#each group.vars as v}<div class="state-item"><button class="token" onclick={()=>toggle(group.name==="当前帧"?{name:v.name,fn:s.frames[0]?.location.function??s.location.function}:{name:v.name,global:true})}>{v.name}</button><DataValue value={v}/></div>{:else}<span class="muted">无已记录值</span>{/each}</div></div>
      {/each}
    </div></td></tr>
  {/if}
{/snippet}
{#snippet trace(items: TraceNode[], indent:number)}
  {#each items as n}
    {#if n.kind==="observation"}{@render snapshot(n.snapshot,indent)}
    {:else}
      <tr class="group"><td colspan={3+columns.length}><button class="group-button" style:padding-left={indent*16+8+"px"} onclick={()=>expand(n.key)} aria-expanded={open.includes(n.key)}>{open.includes(n.key)?"▾":"▸"} {n.label}</button></td></tr>
      {#if open.includes(n.key)}{@render trace(n.children,indent+1)}{/if}
    {/if}
  {/each}
{/snippet}

<header>
  <div><strong>C Observer</strong><small>下一代 GUI · 离线回放原型</small></div>
  <label class="file">打开观察记录 <input type="file" accept=".json,application/json" onchange={load}/></label>
  <span class="spacer"></span><span class="muted">{doc.synthetic?"合成夹具":"导入记录"} · {doc.run.reason} · {doc.run.snapshots.length} 条观察</span>
</header>
{#if error}<div role="alert" class="error">{error}</div>{/if}
<nav>
  <button class:active={mode==="trace"} onclick={()=>mode="trace"}>执行文档</button>
  <button class:active={mode==="source"} onclick={()=>mode="source"}>完整源码</button>
  <button class:active={mode==="io"} onclick={()=>mode="io"}>输入输出</button>
</nav>
{#if mode!=="io"}
  <div class="controls">
    <span class="muted">观察列</span>
    {#each columns as c (columnKey(c))}<button class="chip" onclick={()=>toggle(c)}>{c.fn?c.fn+"::":c.global?"全局::":""}{c.name} ×</button>{/each}
    <select aria-label="添加变量" bind:value={selectedColumn} onchange={()=>{const c=names.find(c=>columnKey(c)===selectedColumn);if(c&&!columns.some(x=>columnKey(x)===selectedColumn))toggle(c);selectedColumn="";}}>
      <option value="">+ 变量</option>{#each names as c}<option value={columnKey(c)}>{c.fn?c.fn+"::":c.global?"全局::":""}{c.name}</option>{/each}
    </select>
    {#if mode==="trace"}<label class="group-toggle"><input type="checkbox" bind:checked={grouped}/>折叠重复与调用</label>{/if}
  </div>
{/if}
{#if mode==="trace"}
  <main class="scroll">
    {#if filter}<div class="filter">仅看 {filter.file.split(/[\\/]/).at(-1)}:{filter.line} · {matching.length} 次观察 <button onclick={()=>filter=null}>清除筛选</button></div>{/if}
    <table class="trace"><thead><tr><th>次序</th><th>行号</th><th>源码 · 暂停位置</th>{#each columns as c (columnKey(c))}<th>{c.name}<small>{c.fn??(c.global?"全局":"按名称")}</small></th>{/each}</tr></thead>
      <tbody>{@render trace(nodes,0)}{#if !matching.length}<tr><td colspan={3+columns.length}>此范围没有已采集的观察记录。</td></tr>{/if}</tbody>
    </table>
  </main>
{:else if mode==="source"}
  <main class="scroll source">
    <select aria-label="源码文件" bind:value={selectedPath}>{#each doc.run.session.sources as s}<option value={s.path}>{s.path}</option>{/each}</select>
    {#each lines as tokens,i}<div class="source-row"><button class="line" onclick={()=>selectLine({file:selectedPath,line:i+1})}>{i+1}</button>{@render code(tokens,contextAt(i+1))}<small>{lineCount(i+1)||""}</small></div>{/each}
  </main>
{:else}
  <main class="scroll io">
    <div class="notice">{doc.expected===null?"未提供预期输出":doc.run.output.stdout_truncated?"实际输出截断，无法完整比较":rawEqual?"显示文本一致":"显示文本不同"} · 样例规则：{doc.comparison||"exact"}（原型仅显示文本差异，非 OJ 判定）</div>
    <section><h2>输入</h2><pre>{doc.stdin}</pre></section>
    <div class="io-grid"><section><h2>预期输出</h2><pre>{doc.expected??"未提供"}</pre></section><section><h2>实际输出 {doc.run.output.stdout_truncated?"· 截断":""}</h2><pre>{actual}</pre></section></div>
    <section><h2>stderr {doc.run.output.stderr_truncated?"· 截断":""}</h2><pre>{stderr||"（空）"}</pre></section>
    <section><h2>运行状态</h2><p>{doc.run.reason} {doc.run.exit_code??""} {doc.run.message??""} {doc.run.cleanup_error??""}</p>{#each doc.run.diagnostics??[] as d}<p>{d.severity}: {d.message}</p>{/each}</section>
  </main>
{/if}
<footer>观察值来自已记录暂停状态；绿色为相邻原始观察的可比较差异（不是赋值归因）。— 无对应记录；分组仅供阅读。{doc.synthetic?" 合成数据，不代表真实运行。":""}</footer>
<style>
  :global(*){box-sizing:border-box}
  :global(body){margin:0;font:14px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--text);background:var(--bg);--bg:#f8f9fb;--panel:#fff;--text:#20242b;--muted:#687383;--line:#dce1e8;--accent:#245ab6;--soft:#eaf1ff;--change:#dff3e7;color-scheme:light}
  @media(prefers-color-scheme:dark){:global(body){--bg:#15191f;--panel:#20262e;--text:#e5e9ef;--muted:#a1acb9;--line:#39434e;--accent:#92b8ff;--soft:#273a56;--change:#244b38;color-scheme:dark}}
  :global(#app){height:100vh;display:flex;flex-direction:column;min-height:0}
  header,nav,.controls,footer{display:flex;align-items:center;gap:8px;padding:10px 16px;border-bottom:1px solid var(--line);background:var(--panel);flex-wrap:wrap}
  header strong{font-size:18px} header small{margin-left:10px}
  small,.muted,footer{color:var(--muted)} .spacer{flex:1}
  button,select,.file{font:inherit;color:inherit;border:1px solid var(--line);background:var(--panel);padding:5px 10px;border-radius:5px;cursor:pointer}
  button:hover,.file:hover{border-color:var(--accent)} button:disabled{cursor:default;opacity:.5}
  .file input{position:absolute;width:1px;height:1px;opacity:0}
  .file:focus-within{outline:2px solid var(--accent)}
  .active,.chip,.selected{background:var(--soft);color:var(--accent)}
  .group-toggle{margin-left:auto;display:flex;align-items:center;gap:6px}
  .scroll{flex:1;min-height:0;overflow:auto}
  .trace{border-collapse:separate;border-spacing:0;min-width:100%;background:var(--panel)}
  .trace th,.trace td{padding:7px 10px;text-align:left;border-bottom:1px solid var(--line);vertical-align:top}
  .trace th{position:sticky;top:0;background:var(--bg);z-index:1;white-space:nowrap;font-weight:600}
  .trace th small{display:block;font-weight:400}
  .trace td:not(.source-cell){font-variant-numeric:tabular-nums}
  .source-cell{min-width:340px}.source-cell small{display:block;font-size:11px}
  .code{font:13px/1.65 ui-monospace,SFMono-Regular,Consolas,monospace;white-space:pre;display:inline}
  .token,.line,.inline-control{border:0;background:transparent;padding:0;font:inherit;color:var(--accent);border-radius:2px}
  .token:hover,.line:hover,.inline-control:hover{text-decoration:underline}
  .token.selected{background:var(--soft)} .line{min-width:32px;text-align:right}
  .keyword{color:var(--accent);font-weight:600}.comment{color:var(--muted)}.literal{color:var(--muted)}
  .seq{color:var(--muted);width:48px}.changed{background:var(--change)}
  .group td{padding:0;background:var(--bg)}.group-button{width:100%;text-align:left;border:0;background:none;font-weight:600;color:var(--accent);padding:9px}
  .detail{padding:12px;border-left:3px solid var(--accent);background:var(--bg);display:grid;gap:10px}
  .state-grid{display:flex;gap:8px;flex-wrap:wrap}.state-item{display:grid;gap:4px;border:1px solid var(--line);padding:8px;background:var(--panel);border-radius:5px}
  .source{padding:12px}.source-row{display:flex;gap:16px;align-items:baseline;min-width:max-content;padding:2px 6px}.source-row:hover{background:var(--soft)}.source-row small{margin-left:auto}
  .filter,.notice,.error{padding:10px 16px;background:var(--soft)}.error{color:#c2410c}
  .io{padding:16px}.io-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.io h2{font-size:14px}.io pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--panel);border:1px solid var(--line);padding:12px;min-height:50px}
  footer{font-size:12px;border-top:1px solid var(--line);border-bottom:0}
</style>
