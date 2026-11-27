<script lang="ts">
  import example from "../../../testdata/snapshots/wrong-index.json";
  import DataValue from "./DataValue.svelte";
  import CompactValue from "./CompactValue.svelte";
  import { parseDocument, tokenize, groupTrace, sourceDocument, groupDifferences, replacedLines, smallLayout, projectRounds, slotSummary, slotHeaders, flatten, label, values, previousValue, columnKey, columnLabel, outputText, MAX_BYTES, type Column, type Location, type Snapshot, type Variable, type TraceNode, type GroupNode } from "./observation";
  let doc=$state(parseDocument(example));
  let mode=$state<"source"|"trace"|"io">("source");
  let columns=$state<Column[]>([{name:"i",fn:"main"},{name:"index",fn:"main"},{name:"x",fn:"main"},{name:"a",fn:"main"}]);
  let focus=$state<{file:string;line:number;key:string}|null>(null);
  let open=$state<string[]>([]);
  let closed=$state<string[]>([]);
  let rounds=$state<Record<string,number>>({});
  let fullLoops=$state<string[]>([]);
  let sourceWidth=$state(520);
  let drag:{x:number;width:number}|null=null;
  let detail=$state<string|null>(null);
  let inspect=$state<{value:Variable;previous?:Variable;column:Column;id:number}|null>(null);
  let dialog:HTMLDialogElement;
  let menu=$state(false);
  let error=$state("");
  let selectedPath=$state(example.run.session.sources[0]?.path??"");
  let sourceTokens=$derived(new Map(doc.run.session.sources.map(s=>[s.path,tokenize(s.text)])));
  let tree=$derived(groupTrace(doc.run));
  let projection=$derived(projectRounds(tree,rounds,fullLoops));
  let document=$derived(sourceDocument(doc.run,selectedPath,projection.nodes));
  let hidden=$derived(replacedLines(document.rows,n=>fullLoops.includes(n.key)&&expanded(n)));
  let layouts=$derived(new Map(columns.map(c=>[columnKey(c),smallLayout(doc.run.snapshots.flatMap(s=>values(s,c)))])));
  let widths=$derived(columns.map(c=>{
    const slots=layouts.get(columnKey(c))??[];
    return slots.length?Math.max(92,slots.length*72):Math.max(92,Math.min(290,columnLabel(c).length*9+38),doc.run.snapshots.some(s=>values(s,c).some(v=>v.children?.length))?250:0);
  }));
  let tableWidth=$derived(75+sourceWidth+widths.reduce((a,b)=>a+b,0));
  let names=$derived([...new Map<string,Column>(doc.run.snapshots.flatMap(s=>[
    ...(s.frames[0]?.variables??[]).map(v=>({name:v.name,fn:s.frames[0]?.location.function??s.location.function})),
    ...(s.globals??[]).map(v=>({name:v.name,global:true}))
  ]).map(c=>[columnKey(c),c] as [string,Column])).values()]);
  let selectedColumn=$state("");
  let matching=$derived(focus?doc.run.snapshots.filter(s=>s.location.file===focus!.file&&s.location.line===focus!.line):[]);
  let actual=$derived(outputText(doc.run.output.stdout));
  let stderr=$derived(outputText(doc.run.output.stderr));
  let equal=$derived(doc.expected!==null&&!doc.run.output.stdout_truncated&&actual===doc.expected);
  const reasons:Record<string,string>={exited:"运行结束",nonzero_exit:"非零退出",signaled:"异常终止",compile_failed:"编译失败",limit_reached:"采集上限",timed_out:"超时",cancelled:"已取消",backend_failed:"采集失败",invalid_config:"配置错误"};
  async function load(e:Event){
    const input=e.currentTarget as HTMLInputElement,file=input.files?.[0];if(!file)return;
    try{
      if(file.size>MAX_BYTES)throw Error("文件超过 128 MiB");
      const next=parseDocument(JSON.parse(await file.text()));
      outputText(next.run.output.stdout);outputText(next.run.output.stderr);
      doc=next;columns=[];open=[];closed=[];rounds={};fullLoops=[];detail=null;focus=null;inspect=null;selectedColumn="";
      selectedPath=next.run.session.sources[0]?.path??"";mode="source";error="";
    }catch(e){error=e instanceof Error?e.message:String(e);}
    input.value="";
  }
  function add(c:Column){if(!columns.some(x=>columnKey(x)===columnKey(c)))columns=[...columns,c];}
  function toggle(c:Column){const k=columnKey(c);if(columns.some(x=>columnKey(x)===k))columns=columns.filter(x=>columnKey(x)!==k);else add(c);}
  function choose(name:string,fn:string|undefined){
    const candidates=names.filter(c=>c.name===name);
    const c=candidates.find(x=>x.fn===fn&&!x.global)??(candidates.length===1?candidates[0]:undefined)??candidates.find(x=>x.global);
    if(c)toggle(c);else if(candidates.length)toggle({name,fn});
  }
  function selectable(name:string){return names.some(c=>c.name===name);}
  function selectLine(loc:Location,key:string){focus=focus?.key===key?null:{file:loc.file,line:loc.line,key};detail=null;}
  function inspectValue(s:Snapshot,c:Column,v:Variable){inspect={value:v,previous:previousValue(doc.run,s,c),column:c,id:s.id};dialog.showModal();}
  function allGroupKeys(ns:TraceNode[]):string[]{return ns.flatMap(n=>n.kind==="group"?[n.key,...allGroupKeys(n.children)]:[]);}
  function expanded(n:{key:string;groupType:string}){return open.includes(n.key)||n.groupType==="loop"&&!closed.includes(n.key);}
  function toggleGroup(n:{key:string;groupType:string}){
    focus=null;
    if(expanded(n)){open=open.filter(k=>k!==n.key);closed=[...closed,n.key];}
    else{closed=closed.filter(k=>k!==n.key);open=[...open,n.key];}
  }
  function stepRound(n:GroupNode,delta:number){
    rounds={...rounds,[n.key]:Math.max(0,Math.min(n.children.length-1,(rounds[n.key]??0)+delta))};
    focus=null;detail=null;
  }
  function toggleFull(n:GroupNode){
    if(fullLoops.includes(n.key))fullLoops=fullLoops.filter(k=>k!==n.key);
    else {fullLoops=[...fullLoops,n.key];closed=closed.filter(k=>k!==n.key);}
    focus=null;detail=null;
  }
  function resize(width:number){sourceWidth=Math.max(240,Math.min(1000,width));}
  function startResize(e:PointerEvent){drag={x:e.clientX,width:sourceWidth};(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);}
  function moveResize(e:PointerEvent){if(drag)resize(drag.width+e.clientX-drag.x);}
  function stopResize(){drag=null;}
  function tokensAt(s:Snapshot){return sourceTokens.get(s.location.file)?.[s.location.line-1]??[];}
  function showDetail(s:Snapshot,key:string){const id=key+"-"+s.id;detail=detail===id?null:id;}
</script>
{#snippet code(tokens,fn)}
  <span class="code">{#each tokens as t}{#if t.kind==="identifier"&&selectable(t.text)}<button class="token" class:selected={columns.some(c=>c.name===t.text&&(!c.fn||c.fn===fn))} onclick={()=>choose(t.text,fn)} title="添加或移除观察列">{t.text}</button>{:else}<span class={t.kind}>{t.text}</span>{/if}{/each}</span>
{/snippet}
{#snippet data(s)}
  {#each columns as c (columnKey(c))}
    <td class="value">{#each values(s,c) as v}<CompactValue value={v} previous={previousValue(doc.run,s,c)} slots={layouts.get(columnKey(c))??[]} onInspect={()=>inspectValue(s,c,v)}/>{:else}<span class="muted" title="当前记录无对应值">—</span>{/each}</td>
  {/each}
{/snippet}
{#snippet state(s,key)}
  {#if detail===key+"-"+s.id}
    <tr><td colspan={3+columns.length}><div class="detail">
      <strong>观察 #{s.id+1} · 暂停状态</strong>
      {#each [{name:"当前帧",vars:s.frames[0]?.variables??[]},{name:"全局/静态",vars:s.globals??[]}] as g}
        <div class="state-grid"><small>{g.name}</small>{#each g.vars as v}
          {@const c=g.name==="当前帧"?{name:v.name,fn:s.frames[0]?.location.function??s.location.function}:{name:v.name,global:true}}
          <div class="state-item"><button class="token" onclick={()=>toggle(c)}>{v.name}</button><DataValue value={v} column={c} onSelect={add}/></div>
        {:else}<span class="muted">无已记录值</span>{/each}</div>
      {/each}
    </div></td></tr>
  {/if}
{/snippet}
{#snippet snapshot(s,indent,context,loopLine)}
  <tr class="observed">
    <td class="seq"><button class="plain" title="展开完整状态" onclick={()=>showDetail(s,"obs")}>{s.id+1}</button></td>
    <td class="gutter"><button class="line" title="查看本行全部观察" onclick={()=>selectLine(s.location,"obs-"+s.id)}>{s.location.line||"—"}</button></td>
    <td class="source-cell"><div class="code-line" style:padding-left={indent*12+"px"}>{#if context}<small class="context">{s.location.function??""} {s.location.file.split(/[\\/]/).at(-1)??""}</small>{/if}{#if loopLine===s.location.line}<span class="muted" title={tokensAt(s).map(t=>t.text).join("")}>↳ 循环头观察</span>{:else if tokensAt(s).length}{@render code(tokensAt(s),s.location.function)}{:else}<span class="muted">{s.location.reported_file??"无可映射源码"}</span>{/if}</div></td>
    {@render data(s)}
  </tr>
  {@render state(s,"obs")}
  {@render lineQuery("obs-"+s.id)}
{/snippet}
{#snippet lineQuery(key)}
  {#if focus?.key===key}
    <tr class="query"><td colspan={3+columns.length}><span>{focus.line} 行 · {matching.length} 次观察</span> <button class="plain" onclick={()=>focus=null}>关闭 ×</button>{#if !matching.length}<small> · 无观察记录，不代表未执行</small>{/if}</td></tr>
    {#each matching as q}<tr class="query"><td class="seq"><button class="plain" onclick={()=>showDetail(q,"query")}>{q.id+1}</button></td><td class="gutter">{q.location.line}</td><td class="muted">本行观察 · #{q.id+1}</td>{@render data(q)}</tr>{@render state(q,"query")}{/each}
  {/if}
{/snippet}
{#snippet summary(n)}
  {#each columns as c}{@const ds=groupDifferences(doc.run,n,c,2)}{@const first=flatten(n.children)[0]}{@const slots=layouts.get(columnKey(c))??[]}<td class="summary value">{#if slots.length}<div class="slot-summary" style:grid-template-columns={"repeat("+slots.length+",minmax(40px,1fr))"}>{#each slots as slot}{@const v=slotSummary(doc.run,n,c,slot)}<span class:diff={v.changed} title={v.title}>{v.text}</span>{/each}</div>{:else}{#if ds.length}{#each ds as d}<span>{d.path} {d.before} → {d.after}</span>{/each}{:else if first}{@const v=values(first,c)}{#if v.length===1&&!v[0].children?.length&&v[0].status==="observed"}<span class="muted" title="组首观察值">{label(v[0])}</span>{/if}{/if}{/if}</td>{/each}
{/snippet}
{#snippet group(n,indent)}
  <tr class="group">
    <td class="seq"></td><td class="gutter"></td>
    <td class="source-cell"><button class="group-button" style:padding-left={indent*12+"px"} onclick={()=>toggleGroup(n)} aria-expanded={expanded(n)}>{expanded(n)?"▾":"▸"} {n.label}</button></td>
    {@render summary(n)}
  </tr>
  {#if expanded(n)}
    {#each n.children as child}
      {#if child.kind==="observation"}{@render snapshot(child.snapshot,indent+1,n.groupType==="call",n.groupType==="round"?n.anchor.line:0)}
      {:else}{@render group(child,indent+1)}{/if}
    {/each}
  {/if}
{/snippet}
<header>
  <strong>C Observer</strong>
  <label class="file">打开记录<input type="file" accept=".json,application/json" onchange={load}/></label>
  <span class="spacer"></span>
  <button class:active={mode==="source"} onclick={()=>{mode="source";focus=null;}}>动态源码</button>
  <button class:active={mode==="trace"} onclick={()=>{mode="trace";focus=null;}}>时间顺序</button>
  <button class:active={mode==="io"} onclick={()=>mode="io"}>输入输出</button>
  <span class="spacer"></span>
  <small>{doc.synthetic?"合成数据 · ":""}{reasons[doc.run.reason]??doc.run.reason} · {doc.run.snapshots.length} 次观察</small>
</header>
{#if error}<div role="alert" class="error">{error}</div>{/if}
{#if mode!=="io"}
  <div class="controls">
    {#if mode==="source"&&doc.run.session.sources.length===1}<span class="filename">{selectedPath.split(/[\\/]/).at(-1)}</span>{:else if mode==="source"}<select aria-label="源码文件" bind:value={selectedPath} onchange={()=>focus=null}>{#each doc.run.session.sources as s}<option value={s.path}>{s.path.split(/[\\/]/).at(-1)}</option>{/each}</select>{/if}
    <select aria-label="添加变量" bind:value={selectedColumn} onchange={()=>{const c=names.find(c=>columnKey(c)===selectedColumn);if(c)add(c);selectedColumn="";}}>
      <option value="">+ 观察变量</option>{#each names as c}<option value={columnKey(c)}>{c.fn?c.fn+"::":c.global?"全局::":""}{c.name}</option>{/each}
    </select>
    
    <span class="spacer"></span>
    {#if mode==="source"}<div class="menu"><button onclick={()=>menu=!menu} aria-expanded={menu} title="分组操作">⋯</button>{#if menu}<div class="menu-items"><button onclick={()=>{fullLoops=[];open=[];closed=allGroupKeys(tree);menu=false;focus=null;}}>返回逐组观察</button><button onclick={()=>{fullLoops=[];rounds={};open=[];closed=[];menu=false;focus=null;}}>恢复默认</button></div>{/if}</div>{/if}
  </div>
  <main class="scroll">
    <table class="trace" style:width={tableWidth+"px"} style:--source-width={sourceWidth+"px"}>
      <colgroup><col style:width="37px"/><col style:width="38px"/><col style:width={sourceWidth+"px"}/>{#each widths as w}<col style:width={w+"px"}/>{/each}</colgroup>
      <thead><tr><th class="seq">#</th><th class="gutter">行</th><th class="source-cell">源码<button class="resize" aria-label="调整源码列宽度" title="拖动调整源码宽度，也可用左右方向键" onpointerdown={startResize} onpointermove={moveResize} onpointerup={stopResize} onpointercancel={stopResize} onlostpointercapture={stopResize} onkeydown={(e)=>{if(e.key==="ArrowLeft"||e.key==="ArrowRight"){e.preventDefault();resize(sourceWidth+(e.key==="ArrowLeft"?-24:24));}}}></button></th>
        {#each columns as c (columnKey(c))}{@const heads=slotHeaders(layouts.get(columnKey(c))??[])}<th class="value" title={c.fn??(c.global?"全局":"按名称")}>{columnLabel(c)+heads.suffix} <button class="plain remove" title="移除观察列" onclick={()=>toggle(c)}>×</button>{#if layouts.get(columnKey(c))?.length}<div class="slot-head" style:grid-template-columns={"repeat("+layouts.get(columnKey(c))!.length+",minmax(40px,1fr))"}>{#each heads.labels as title}<span {title}>{title}</span>{/each}</div>{/if}</th>{/each}
      </tr></thead>
      <tbody>
        {#if mode==="trace"}
          {#each doc.run.snapshots as s,i}{@render snapshot(s,0,i===0||doc.run.snapshots[i-1].location.function!==s.location.function||doc.run.snapshots[i-1].location.file!==s.location.file,0)}{/each}
          {#if !doc.run.snapshots.length}<tr><td colspan={3+columns.length}>没有观察记录</td></tr>{/if}
        {:else}
          {#each document.rows as r}
            {#if !hidden.has(r.line)}
              {@const loops=projection.loops.filter(n=>n.anchor.file===selectedPath&&n.anchor.line===r.line)}
              {@const loop=loops[0]}
              {@const direct=r.nodes.find(n=>n.kind==="observation")?.snapshot}
              <tr class:static-row={!direct} class:loop-head={!!loop}>
                <td class="seq">{#if direct}<button class="plain" title="展开完整状态" onclick={()=>showDetail(direct,"src")}>{direct.id+1}</button>{/if}</td>
                <td class="gutter"><button class="line" title={r.count+" 次观察"} onclick={()=>selectLine({file:selectedPath,line:r.line},"src-"+r.line)}>{r.line}</button></td>
                <td class="source-cell"><div class="code-line"><span class="fold-space"></span>{@render code(r.tokens,r.fn)}</div>
                  {#each loops as lp}
                    <div class="round-controls">
                      {#if loops.length>1}<small>片段 #{flatten(lp.children)[0]?.id+1}</small>{/if}
                      {#if !fullLoops.includes(lp.key)}
                        <button class="round-arrow" disabled={(rounds[lp.key]??0)===0} onclick={()=>stepRound(lp,-1)} aria-label="上一组">‹</button>
                        <select class="round-select" aria-label="选择观察组" value={rounds[lp.key]??0} onchange={(e)=>{rounds={...rounds,[lp.key]:Number(e.currentTarget.value)};focus=null;detail=null;}}>
                          {#each lp.children as _,i}<option value={i}>第 {i+1} / {lp.children.length} 组</option>{/each}
                        </select>
                        <button class="round-arrow" disabled={(rounds[lp.key]??0)>=lp.children.length-1} onclick={()=>stepRound(lp,1)} aria-label="下一组">›</button>
                      {/if}
                      <button class="round-all" onclick={()=>toggleFull(lp)}>{fullLoops.includes(lp.key)?"逐组":"展开全部"}</button>
                    </div>
                  {/each}
                  {#if !loops.length&&r.nodes.length>1}<button class="count" onclick={()=>selectLine({file:selectedPath,line:r.line},"src-"+r.line)}>{r.count} 次</button>{/if}
                </td>
                {#if direct}{@render data(direct)}{:else if loop&&fullLoops.includes(loop.key)}{@render summary(loop)}{:else}{#each columns as _}<td class="value"></td>{/each}{/if}
              </tr>
              {#if direct}{@render state(direct,"src")}{/if}
              {@render lineQuery("src-"+r.line)}
              {#each r.nodes as n}
                {#if n.kind==="group"&&n.groupType==="loop"&&fullLoops.includes(n.key)}
                  {#if expanded(n)}{#each n.children as child}{#if child.kind==="group"}{@render group(child,1)}{:else}{@render snapshot(child.snapshot,1,false,0)}{/if}{/each}{/if}
                {:else if n.kind==="group"}{@render group(n,1)}
                {:else if n.snapshot.id!==direct?.id}{@render snapshot(n.snapshot,1,false,0)}{/if}
              {/each}
            {/if}
          {/each}
          {#if document.unmapped.length}
            <tr class="group"><td colspan={3+columns.length}>未映射观察 · 可在时间顺序中查看</td></tr>
            {#each document.unmapped as n}{#if n.kind==="observation"}{@render snapshot(n.snapshot,0,true,0)}{:else}{@render group(n,0)}{/if}{/each}
          {/if}
          {#if !document.rows.length&&!document.unmapped.length}<tr><td colspan={3+columns.length}>没有可显示源码</td></tr>{/if}
        {/if}
      </tbody>
    </table>
  </main>
{:else}
  <main class="scroll io">
    <div class="notice">{doc.expected===null?"未提供预期输出":doc.run.output.stdout_truncated?"实际输出截断":equal?"显示文本一致":"显示文本不同"} · 样例规则：{doc.comparison||"exact"}（仅文本对照，非 OJ 判定）</div>
    <section><h2>输入</h2><pre>{doc.stdin}</pre></section>
    <div class="io-grid"><section><h2>预期输出</h2><pre>{doc.expected??"未提供"}</pre></section><section><h2>实际输出 {doc.run.output.stdout_truncated?"· 截断":""}</h2><pre>{actual}</pre></section></div>
    <section><h2>stderr {doc.run.output.stderr_truncated?"· 截断":""}</h2><pre>{stderr||"（空）"}</pre></section>
    <section><h2>运行状态</h2><p>{doc.run.reason} {doc.run.exit_code??""} {doc.run.message??""} {doc.run.cleanup_error??""}</p>{#each doc.run.diagnostics??[] as d}<p>{d.severity}: {d.message}</p>{/each}</section>
  </main>
{/if}
<dialog bind:this={dialog} onclose={()=>inspect=null}>
  {#if inspect}<div class="dialog-head"><strong>{columnLabel(inspect.column)}</strong><small>观察 #{inspect.id+1} · {inspect.value.type}</small><span class="spacer"></span><button onclick={()=>dialog.close()} aria-label="关闭详情">×</button></div><div class="dialog-content"><DataValue value={inspect.value} previous={inspect.previous} column={inspect.column} onSelect={add}/></div><div class="dialog-foot">点击成员旁的 + 添加观察列。</div>{/if}
</dialog>
<footer title="数值是实际暂停观察，蓝色为相邻原始观察的差异，不是赋值归因；分组仅为阅读提示。">暂停观察 · 空白：本组无记录 · ?：未采集 · —：无对应值 · 蓝色：观察值变化 {doc.synthetic?" · 合成记录":""}</footer>
<style>
  :global(*){box-sizing:border-box}
  :global(body){margin:0;font:13px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--text);background:var(--bg);--bg:#f7f8fa;--panel:#fff;--text:#20242b;--muted:#697484;--line:#e2e6ec;--accent:#245ab6;--soft:#eaf1ff;color-scheme:light}
  @media(prefers-color-scheme:dark){:global(body){--bg:#161a20;--panel:#20262e;--text:#e5e9ef;--muted:#a1acb9;--line:#39434e;--accent:#92b8ff;--soft:#273a56;color-scheme:dark}}
  :global(#app){height:100vh;display:flex;flex-direction:column;min-height:0}
  header,.controls,footer{display:flex;align-items:center;gap:8px;padding:6px 12px;border-bottom:1px solid var(--line);background:var(--panel);flex-wrap:wrap}
  header strong{font-size:16px}small,.muted,footer{color:var(--muted)}.spacer{flex:1}
  button,select,.file{font:inherit;color:inherit;border:1px solid var(--line);background:var(--panel);padding:3px 8px;border-radius:4px;cursor:pointer}
  button:hover,.file:hover{border-color:var(--accent)}
  .file input{position:absolute;width:1px;height:1px;opacity:0}.file:focus-within{outline:2px solid var(--accent)}
  .active,.selected{background:var(--soft);color:var(--accent)}
  .scroll{flex:1;min-height:0;overflow:auto;background:var(--panel)}
  .trace{border-collapse:separate;border-spacing:0;table-layout:fixed;background:var(--panel)}
  .trace th,.trace td{padding:3px 8px;text-align:left;vertical-align:top;height:26px}
  .trace th{position:sticky;top:0;background:var(--bg);z-index:2;white-space:nowrap;font-weight:600}
  .seq{min-width:37px;width:37px;color:var(--muted);text-align:right!important;font-variant-numeric:tabular-nums}
  .gutter{min-width:38px;width:38px;text-align:right!important}
  .source-cell{width:var(--source-width);overflow-wrap:anywhere}
  .code-line{white-space:pre-wrap;overflow-wrap:anywhere}
  .code{font:13px/1.5 ui-monospace,SFMono-Regular,Consolas,monospace;white-space:pre-wrap;tab-size:4}
  .token,.line,.plain,.count{border:0;background:transparent;padding:0;font:inherit;color:var(--accent);border-radius:2px}
  .token{color:inherit}.token.selected{background:var(--soft);color:var(--accent)}
  .token:hover,.line:hover,.plain:hover,.count:hover{text-decoration:underline}
  .line{color:var(--muted)}.seq button{color:var(--muted)}.remove{margin-left:4px;color:var(--muted)}
  .count{margin-left:8px;color:var(--accent);font-size:11px;white-space:nowrap}
  .keyword{color:var(--accent);font-weight:600}.comment,.literal{color:var(--muted)}
  .context{display:inline-block;margin-right:8px;font:11px system-ui}
  .static-row .source-cell{color:var(--text)}
  .value{border-left:1px solid var(--line);font-variant-numeric:tabular-nums;overflow-wrap:anywhere}
  .group td{background:var(--bg);padding-top:3px;padding-bottom:3px}.group td:nth-child(3){border-left:2px solid var(--line)}
  .group-button{width:100%;text-align:left;border:0;background:none;font-weight:600;color:var(--accent);padding:0;white-space:normal}
  .summary{color:var(--accent);font-size:12px;max-width:420px}.summary span{display:inline-block;margin-right:6px}
  .detail{padding:8px;border-left:2px solid var(--accent);background:var(--bg);display:grid;gap:6px}
  .state-grid{display:flex;gap:6px;align-items:baseline;flex-wrap:wrap}.state-item{display:grid;gap:2px;border:1px solid var(--line);padding:5px;background:var(--panel);border-radius:4px}
  .focus,.notice,.error{padding:8px 12px;background:var(--soft)}.error{color:#c2410c}.focus button{margin-left:8px}.focus small{margin-left:8px}
  .io{padding:12px}.io-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.io h2{font-size:14px}.io pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--bg);border:1px solid var(--line);padding:8px;min-height:40px}
  footer{font-size:11px;border-top:1px solid var(--line);border-bottom:0;padding:4px 12px}

  .trace tbody tr:hover{background:var(--bg)}
  .trace th{border-bottom:1px solid var(--line)}
  .fold,.fold-space{display:inline-block;width:14px;padding:0;margin:0 2px 0 0;border:0;background:none;color:var(--accent)}
  .loop-head td{border-top:1px solid var(--line)}
  .slot-head{display:grid;gap:0;font-size:11px;color:var(--muted);font-weight:400}
  .slot-head span{text-align:center;padding:0 4px;border-right:1px solid var(--line);white-space:nowrap}
  .slot-head span:last-child{border:0}
  .query td{background:var(--bg);border-bottom:1px solid var(--line)}
  .menu{position:relative}.menu-items{position:absolute;right:0;top:100%;background:var(--panel);padding:4px;z-index:5;box-shadow:0 4px 12px #0002;border:1px solid var(--line);display:grid;gap:4px;white-space:nowrap}
  dialog{color:var(--text);background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:0;width:min(520px,90vw);max-height:75vh;box-shadow:0 12px 40px #0003}
  dialog::backdrop{background:#0002}
  .dialog-head,.dialog-foot{display:flex;gap:8px;align-items:center;padding:10px 12px;border-bottom:1px solid var(--line)}
  .dialog-content{padding:12px;overflow:auto;max-height:55vh}
  .dialog-foot{border-bottom:0;border-top:1px solid var(--line);font-size:12px;color:var(--muted)}

  .filename{font-weight:500;padding:2px 5px}
  .trace .seq,.trace .gutter,.trace .source-cell{position:sticky;background:var(--panel);z-index:1}
  .trace .seq{left:0}.trace .gutter{left:37px}.trace .source-cell{left:75px;box-shadow:1px 0 var(--line)}
  .trace th.seq,.trace th.gutter,.trace th.source-cell{z-index:4;background:var(--bg)}
  .trace th.value{z-index:2}
  .trace tr:hover td.seq,.trace tr:hover td.gutter,.trace tr:hover td.source-cell{background:var(--bg)}
  .trace .group td.seq,.trace .group td.gutter,.trace .group td.source-cell{background:var(--bg)}
  .trace .value{font-family:ui-monospace,SFMono-Regular,Consolas,monospace}
  .resize{position:absolute;right:0;top:0;bottom:0;width:7px;border:0;border-radius:0;padding:0;background:transparent;cursor:col-resize;touch-action:none}
  .resize:hover,.resize:focus-visible{background:var(--accent)}
  .round-controls{display:flex;align-items:center;gap:4px;margin-left:16px;min-height:24px;white-space:nowrap}
  .round-arrow,.round-all,.round-select{padding:1px 5px;font-size:12px;color:var(--accent)}
  .round-arrow:disabled{opacity:.35;cursor:default}.round-select{max-width:135px}
  .slot-summary{display:grid;width:100%;gap:0;font-variant-numeric:tabular-nums}
  .slot-summary span{margin:0;padding:0 3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;text-align:center;border-right:1px solid var(--line);color:var(--muted)}
  .slot-summary span:last-child{border:0}.slot-summary .diff{color:var(--accent);background:var(--soft);font-weight:600}
  .slot-head span{overflow:hidden;text-overflow:ellipsis}
  .trace td.value{overflow:hidden}
</style>
