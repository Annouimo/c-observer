<script lang="ts">
  import example from "../../../testdata/snapshots/wrong-index.json";
  import DataModule from "./DataModule.svelte";
  import {parseDocument,modules,selectedValues,matchingPrevious,changeEvents,nextEvent,columnKey,columnLabel,outputText,MAX_BYTES,visibleModules,isCompound,moduleWidth,replayContext,markerPath,sampleResult,outputRows,sourceTokens,type Column} from "./observation";
  let doc=$state(parseDocument(example));
  let step=$state(0);
  let selected=$state<Column|null>(null);
  let error=$state("");
  let dialog:HTMLDialogElement;
  let dialogMode=$state<"source"|"io">("io");
  let allModules=$derived(modules(doc.run));
  let list=$derived(visibleModules(allModules,current));
  let eventCache=new Map<string,number[]>();
  let current=$derived(doc.run.snapshots[step]);
  let total=$derived(doc.run.snapshots.length);
  let active=$derived(selected?columnKey(selected):"");
  function cachedEvents(c:Column|null){if(!c)return [];const k=columnKey(c);if(!eventCache.has(k)){if(eventCache.size>=64)eventCache.delete(eventCache.keys().next().value!);eventCache.set(k,changeEvents(doc.run,c));}return eventCache.get(k)!;}
  let events=$derived(cachedEvents(selected));
  let marks=$derived(markerPath(events,total));
  let prevChange=$derived(nextEvent(events,step,-1));
  let nextChange=$derived(nextEvent(events,step,1));
  let interval=$derived(replayContext(doc.run,step));
  let currentSource=$derived(doc.run.session.sources.find(s=>s.path===interval.origin?.file));
  let tokenMap=$derived(new Map(doc.run.session.sources.map(s=>[s.path,sourceTokens(s.text)])));
  let sourceLines=$derived(tokenMap.get(currentSource?.path??"")??[]);
  let line=$derived(interval.origin?.line??0);
  let context=$derived([line-1,line,line+1].map(n=>({tokens:sourceLines[n-1],line:n})));
  function place(loc:typeof interval.origin){return loc?(loc.file||loc.reported_file||"未知位置").split(/[\\/]/).at(-1)+":"+ (loc.line||"?"):"未知位置";}
  let contextTitle=$derived(interval.before?place(interval.before)+" → "+place(interval.after):interval.origin?"初始观察 · "+place(interval.origin):"无观察位置");
  let incomplete=$derived(current?.frames[0]?.variables_complete===false||doc.run.session.capabilities?.globals===true&&current?.globals_complete===false);
  let actual=$derived(outputText(doc.run.output.stdout));
  let stderr=$derived(outputText(doc.run.output.stderr));
  let result=$derived(sampleResult(doc));
  let comparisonRows=$derived(dialogMode==="io"&&doc.expected!==null?outputRows(doc):[]);
  let groups=$derived([...new Set(list.map(m=>m.column.global?"全局 / 静态":m.column.fn??"局部变量"))]);
  let sourcePath=$state("");
  let fullLines=$derived(doc.run.session.sources.find(s=>s.path===sourcePath)?.text.split(/\r?\n/)??[]);
  const reasons:Record<string,string>={exited:"运行结束",nonzero_exit:"非零退出",signaled:"异常终止",compile_failed:"编译失败",limit_reached:"采集上限",timed_out:"超时",cancelled:"已取消",backend_failed:"采集失败",invalid_config:"配置错误"};
  function go(id:number){if(total&&Number.isFinite(id))step=Math.max(0,Math.min(total-1,Math.round(id)));}
  function choose(c:Column){selected=active===columnKey(c)?null:c;}
  function show(mode:"source"|"io"){
    dialogMode=mode;sourcePath=currentSource?.path??doc.run.session.sources[0]?.path??"";dialog.showModal();
  }
  function keys(e:KeyboardEvent){
    if(dialog?.open||e.metaKey||e.ctrlKey||e.altKey||["INPUT","SELECT","TEXTAREA"].includes((e.target as HTMLElement)?.tagName))return;
    if(e.key==="ArrowLeft"||e.key==="ArrowRight"){e.preventDefault();go(step+(e.key==="ArrowLeft"?-1:1));}
  }
  async function load(e:Event){
    const input=e.currentTarget as HTMLInputElement,file=input.files?.[0];if(!file)return;
    try{
      if(file.size>MAX_BYTES)throw Error("文件超过 128 MiB");
      const next=parseDocument(JSON.parse(await file.text()));
      outputText(next.run.output.stdout);outputText(next.run.output.stderr);
      eventCache=new Map();doc=next;step=0;selected=null;error="";
    }catch(e){error=e instanceof Error?e.message:String(e);}
    input.value="";
  }
</script>
<svelte:window onkeydown={keys}/>
<header>
  <strong>C Observer</strong>
  <label class="file">打开记录<input type="file" accept=".json,application/json" onchange={load}/></label>
  <span class="spacer"></span>
  {#if doc.synthetic}<small>合成记录</small>{/if}
  <small>{reasons[doc.run.reason]??doc.run.reason}</small>
  <button class="result" class:bad={result==="输出不一致"||result==="运行未正常结束"||result==="输出不完整"} onclick={()=>show("io")} title="查看输入输出与运行状态">{result} ›</button>
</header>
{#if error}<div class="error" role="alert">{error}</div>{/if}
<section class="context">
  <div class="source-context">
    <div class="context-head"><span title={contextTitle}>{contextTitle}</span><span class="spacer"></span><button class="quiet" onclick={()=>show("source")} disabled={!doc.run.session.sources.length}>源码</button></div>
    <div class="context-lines">
      {#if line>0&&sourceLines[line-1]}
        {#each context as r}<div class="code-line" class:current-line={r.line===line}><span class="line-no">{r.tokens?r.line:""}</span><code>{#each r.tokens??[] as t}<span class={t.kind}>{t.text}</span>{/each}</code></div>{/each}
      {:else}<div class="code-line"><span class="line-no">—</span><code>无可映射的暂停源码</code></div>{/if}
    </div>
  </div>
  <button class="sample" onclick={()=>show("io")} title="本次完整样例输入输出">
    <span class="sample-title">本次样例 <span>完整对比 ›</span></span>
    <span class="sample-row"><small>输入</small><code>{doc.stdin||"（空）"}</code></span>
    <span class="sample-row"><small>预期</small><code>{doc.expected===null?"未提供":doc.expected||"（空）"}</code></span>
    <span class="sample-row" class:bad={result==="输出不一致"}><small>实际</small><code>{actual||"（空）"}</code></span>
  </button>
</section>
<main>
  {#if current}
    {#if incomplete}<div class="capture-note">部分变量未记录</div>{/if}
    {#each groups as group}
      <section class="data-group">
        <h2>{group==="全局 / 静态"?group:groups.length===1?"局部变量":group+" · 局部变量"}</h2>
        <div class="scalars">
          {#each list.filter(m=>(m.column.global?"全局 / 静态":m.column.fn??"局部变量")===group&&!isCompound(m.shape)) as m (columnKey(m.column))}
            {@const vs=selectedValues(current,m.column)}
            <div class="module-slot" style:--ideal={moduleWidth(m.shape)+"px"}><DataModule shape={m.shape} column={m.column} value={vs.length===1?vs[0]:undefined} previous={matchingPrevious(doc.run,current,m.column)} {active} onSelect={choose} root ambiguous={vs}/></div>
          {/each}
        </div>
        <div class="compounds">
          {#each list.filter(m=>(m.column.global?"全局 / 静态":m.column.fn??"局部变量")===group&&isCompound(m.shape)) as m (columnKey(m.column))}
            {@const vs=selectedValues(current,m.column)}
            <div class="module-slot" style:--ideal={moduleWidth(m.shape)+"px"}><DataModule shape={m.shape} column={m.column} value={vs.length===1?vs[0]:undefined} previous={matchingPrevious(doc.run,current,m.column)} {active} onSelect={choose} root ambiguous={vs}/></div>
          {/each}
        </div>
      </section>
    {/each}
    {#if !list.length}<div class="empty">没有已记录变量</div>{/if}
  {:else}<div class="empty">没有观察记录 · {reasons[doc.run.reason]??doc.run.reason}{#if doc.run.message}<p>{doc.run.message}</p>{/if}</div>{/if}
</main>
<footer>
  <div class="timeline-head">
    <div class="selection">{#if selected}<strong>{columnLabel(selected)}</strong><span> · {events.length} 次已知变化</span>{:else}<span>点击数据，定位变化</span>{/if}</div>
    <span class="spacer"></span>
    <span class="legend">观察次序 · — 无对应 · ? 未记录</span>
  </div>
  <div class="timeline">
    <div class="track" aria-hidden="true"></div>
    <svg class="markers" viewBox="0 0 1000 12" preserveAspectRatio="none" aria-hidden="true"><path d={marks} vector-effect="non-scaling-stroke"/></svg>
    <input class="scrubber" aria-label="观察时间线" type="range" min="0" max={Math.max(0,total-1)} step="1" value={step} disabled={total<2} oninput={e=>go(Number(e.currentTarget.value))}/>
  </div>
  <div class="timeline-controls">
    <button class="quiet" disabled={!step} onclick={()=>go(0)} title="首个观察" aria-label="首个观察">|‹</button>
    <button class="quiet" disabled={prevChange===undefined} onclick={()=>prevChange!==undefined&&go(prevChange)} title="上一次已知变化" aria-label="上一次已知变化">«</button>
    <button class="quiet" disabled={!step} onclick={()=>go(step-1)} title="上一观察" aria-label="上一观察">‹</button>
    <span class="spacer"></span>
    <label class="position">观察 <input aria-label="观察编号" type="number" min="1" max={total||1} value={total?step+1:0} disabled={!total} onchange={e=>go(Number(e.currentTarget.value)-1)}/> / {total}</label>
    <span class="spacer"></span>
    <button class="quiet" disabled={step>=total-1} onclick={()=>go(step+1)} title="下一观察" aria-label="下一观察">›</button>
    <button class="quiet" disabled={nextChange===undefined} onclick={()=>nextChange!==undefined&&go(nextChange)} title="下一次已知变化" aria-label="下一次已知变化">»</button>
    <button class="quiet" disabled={step>=total-1} onclick={()=>go(total-1)} title="最后观察" aria-label="最后观察">›|</button>
  </div>
</footer>
<dialog bind:this={dialog}>
  <div class="dialog-head"><strong>{dialogMode==="source"?"冻结源码":"输入输出"}</strong><span class="spacer"></span><button onclick={()=>dialog.close()} aria-label="关闭">×</button></div>
  <div class="dialog-body">
    {#if dialogMode==="source"}
      {#if doc.run.session.sources.length>1}<select aria-label="源码文件" bind:value={sourcePath}>{#each doc.run.session.sources as s}<option value={s.path}>{s.path}</option>{/each}</select>{/if}
      {#each fullLines as text,i}<div class="code-line" class:current-line={sourcePath===current?.location.file&&line===i+1}><span class="line-no">{i+1}</span><code>{text||" "}</code></div>{/each}
    {:else}
      <small>{result} · {doc.comparison||"原始字节"}（样例对照，非 OJ 判定；逐行表显示原始字节差异）</small>
      <h2>输入</h2><pre>{doc.stdin}</pre>
      <div class="outputs"><section><h2>预期</h2><pre>{doc.expected??"未提供"}</pre></section><section><h2>实际 {doc.run.output.stdout_truncated?"· 截断":""}</h2><pre>{actual}</pre></section></div>
      {#if doc.expected!==null}<h2>逐行差异 · · 空格 / ↵ 换行 / \\r 回车</h2><table class="output-table"><thead><tr><th>行</th><th>预期</th><th>实际</th></tr></thead><tbody>{#each comparisonRows as r,i}<tr class:bad-row={!r.equal}><th>{i+1}</th><td>{r.expected??"—"}</td><td>{r.actual??"—"}</td></tr>{/each}</tbody></table>{/if}
      <h2>stderr {doc.run.output.stderr_truncated?"· 截断":""}</h2><pre>{stderr||"（空）"}</pre>
      <h2>运行状态</h2><p>{doc.run.reason} {doc.run.exit_code??""} {doc.run.message??""} {doc.run.cleanup_error??""}</p>
      {#each doc.run.diagnostics??[] as d}<p>{d.severity}: {d.message}</p>{/each}
    {/if}
  </div>
</dialog>
<style>
  :global(*){box-sizing:border-box}
  :global(body){margin:0;font:13px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--text);background:var(--bg);--bg:#f6f7f9;--panel:#fff;--text:#20242b;--muted:#707987;--line:#e0e4eb;--accent:#285cba;--soft:#eaf1ff;--hover:#f0f4fa;--mono:ui-monospace,SFMono-Regular,Consolas,monospace;color-scheme:light}
  @media(prefers-color-scheme:dark){:global(body){--bg:#161a20;--panel:#20262e;--text:#e5e9ef;--muted:#a1acb9;--line:#39434e;--accent:#92b8ff;--soft:#273a56;--hover:#29313b;color-scheme:dark}}
  :global(#app){height:100vh;display:flex;flex-direction:column;min-height:0}
  header,.context,footer{flex:none;background:var(--panel)}
  header{display:flex;align-items:center;gap:10px;padding:7px 14px;border-bottom:1px solid var(--line);min-height:40px}
  header strong{font-size:15px}.spacer{flex:1}small{color:var(--muted);font-size:11px}
  button,select,.file{border:1px solid var(--line);border-radius:4px;padding:3px 8px;color:inherit;font:inherit;background:var(--panel);cursor:pointer}
  button:hover,.file:hover{background:var(--hover)}button:disabled{opacity:.35;cursor:default}
  .file input{position:absolute;opacity:0;width:1px;height:1px}.file:focus-within{outline:2px solid var(--accent)}
  .result{font-size:12px}.bad{color:#b45309}
  .context{display:grid;grid-template-columns:minmax(0,3fr) minmax(180px,2fr);gap:12px;padding:6px 14px 8px;border-bottom:1px solid var(--line)} .source-context{min-width:0} .sample{display:grid;grid-template-rows:17px repeat(3,21px);min-width:0;text-align:left;border:0;border-left:1px solid var(--line);border-radius:0;padding:0 0 0 12px;overflow:hidden}.sample-title{display:flex;justify-content:space-between;color:var(--muted);font-size:11px}.sample-row{display:grid;grid-template-columns:30px minmax(0,1fr);gap:6px;align-items:center;min-width:0}.sample-row code{overflow:hidden;text-overflow:ellipsis;white-space:pre}.keyword{color:var(--accent);font-weight:600}.comment{color:var(--muted)}.literal{color:var(--muted)}
  .context-head{display:flex;align-items:center;color:var(--muted);font-size:11px;gap:8px;margin-bottom:3px}
  .context-head>span:first-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .quiet{border:0;padding:3px 7px;font-size:12px;background:transparent}
  .context-lines{height:63px;overflow:auto}
  .code-line{display:flex;gap:12px;min-height:21px;line-height:21px;align-items:baseline}
  .line-no{flex:0 0 30px;text-align:right;font:11px var(--mono);color:var(--muted)}
  code{font:12px/21px var(--mono);white-space:pre;min-width:0}
  .current-line{background:var(--soft);color:var(--accent);border-radius:3px}
  main{flex:1;min-height:0;overflow:auto;padding:12px 14px 18px}
  .data-group{margin-bottom:13px}h2{font-size:12px;margin:0 0 6px;color:var(--muted);font-weight:600}
  .scalars,.compounds{display:flex;flex-wrap:wrap;align-items:flex-start;gap:6px}
  .module-slot{width:min(100%,var(--ideal));max-width:100%;min-width:0}
  .compounds{gap:8px;margin-top:8px}.compounds .module-slot{flex:0 1 var(--ideal)}
  .capture-note{color:var(--muted);font-size:11px;margin-bottom:6px}
  .scalars:empty,.compounds:empty{display:none}
  .empty{color:var(--muted);padding:20px;text-align:center}
  footer{border-top:1px solid var(--line);padding:7px 14px 5px}
  .timeline-head,.timeline-controls{display:flex;gap:6px;align-items:center;min-height:26px}
  .selection{font-size:12px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .selection strong{color:var(--accent);font:600 12px var(--mono)}
  .timeline{position:relative;height:32px;margin:1px 0}
  .track{position:absolute;left:8px;right:8px;top:15px;height:4px;border-radius:2px;background:var(--line);pointer-events:none}
  .markers{position:absolute;left:8px;width:calc(100% - 16px);top:5px;height:13px;pointer-events:none;overflow:visible}
  .markers path{stroke:var(--accent);stroke-width:1.5;fill:none}
  .scrubber{position:absolute;inset:0;display:block;width:100%;height:32px;margin:0;appearance:none;-webkit-appearance:none;background:transparent;cursor:ew-resize}
  .scrubber::-webkit-slider-runnable-track{height:4px;background:transparent}
  .scrubber::-webkit-slider-thumb{-webkit-appearance:none;width:16px;height:16px;background:var(--accent);border:2px solid var(--panel);border-radius:50%;margin-top:-6px;box-shadow:0 0 0 1px var(--accent)}
  .scrubber::-moz-range-track{height:4px;background:transparent}
  .scrubber::-moz-range-thumb{width:12px;height:12px;background:var(--accent);border:2px solid var(--panel);border-radius:50%}
  .timeline-controls .quiet{font:18px/1 system-ui;min-width:30px}
  .scrubber:disabled{cursor:default}
  .position{font-size:12px;color:var(--muted);white-space:nowrap}
  .position input{width:65px;border:1px solid var(--line);border-radius:4px;background:var(--panel);color:var(--text);font:13px var(--mono);text-align:center;padding:3px}
  .legend{font-size:10px;color:var(--muted);margin-top:3px}
  .error{padding:6px 14px;color:#b45309}
  dialog{color:var(--text);background:var(--panel);border:1px solid var(--line);border-radius:7px;padding:0;width:min(760px,90vw);max-height:85vh}
  dialog::backdrop{background:#0003}
  .dialog-head{display:flex;align-items:center;padding:8px 12px;border-bottom:1px solid var(--line)}
  .dialog-body{padding:12px;max-height:70vh;overflow:auto}
  .dialog-body h2{margin-top:12px}
  .outputs{display:grid;grid-template-columns:1fr 1fr;gap:10px}
  .output-table{border-collapse:collapse;width:100%;font:12px var(--mono);table-layout:fixed}.output-table th:first-child{width:40px}.output-table td,.output-table th{border:1px solid var(--line);padding:5px;text-align:left;white-space:pre-wrap;overflow-wrap:anywhere}.bad-row td{background:var(--soft);color:var(--accent)}
  @media(max-width:620px){.context{grid-template-columns:minmax(0,1fr) minmax(140px,.65fr);gap:6px}.sample{padding-left:6px}.legend{display:none}}
  pre{background:var(--bg);padding:8px;white-space:pre-wrap;overflow-wrap:anywhere}
</style>
