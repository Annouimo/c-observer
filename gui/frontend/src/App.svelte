<script lang="ts">
  import example from "../../../testdata/snapshots/wrong-index.json";
  import DataModule from "./DataModule.svelte";
  import Timeline from "./Timeline.svelte";
  import {parseDocument,modules,selectedValues,matchingPrevious,changeEvents,navigationTarget,pulseTargets,columnKey,columnLabel,outputText,MAX_BYTES,visibleModules,isCompound,moduleWidth,replayContext,sampleResult,outputRows,sourceTokens,samplePreview,sourceMotion,type Column} from "./observation";
  let doc=$state(parseDocument(example));
  let step=$state(0);
  let motion=$state({offset:0,serial:0});
  let pulses=$state<Set<string>>(new Set());
  // Only the small source strip moves. No animation queue and no delayed
  // snapshot state; cancel on new input and respect the OS motion preference.
  function slide(node:HTMLElement,initial:{offset:number;serial:number}){
    let animation:Animation|undefined;
    return {update(next:{offset:number;serial:number}){
      animation?.cancel();
      if(next.offset&&!matchMedia("(prefers-reduced-motion: reduce)").matches&&typeof node.animate==="function"){
        animation=node.animate([{transform:"translateY("+next.offset+"px)"},{transform:"translateY(0)"}],{duration:135,easing:"ease-out"});
      }
    },destroy(){animation?.cancel();}};
  }
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

  let interval=$derived(replayContext(doc.run,step));
  let currentSource=$derived(doc.run.session.sources.find(s=>s.path===interval.origin?.file));
  let tokenMap=$derived(new Map(doc.run.session.sources.map(s=>[s.path,sourceTokens(s.text)])));
  let sourceLines=$derived(tokenMap.get(currentSource?.path??"")??[]);
  let line=$derived(interval.origin?.line??0);
  let context=$derived(Array.from({length:7},(_,i)=>line+i-3).map(n=>({tokens:sourceLines[n-1],line:n})));
  function place(loc:typeof interval.origin){return loc?(loc.file||loc.reported_file||"未知位置").split(/[\\/]/).at(-1)+":"+ (loc.line||"?"):"未知位置";}
  let contextTitle=$derived(interval.before?place(interval.before)+" → "+(interval.before.file===interval.after?.file?interval.after?.line:place(interval.after)):interval.origin?place(interval.origin):"无观察位置");
  let incomplete=$derived(current?.frames[0]?.variables_complete===false||doc.run.session.capabilities?.globals===true&&current?.globals_complete===false);
  let actual=$derived(outputText(doc.run.output.stdout));
  let stderr=$derived(outputText(doc.run.output.stderr));
  let result=$derived(sampleResult(doc));
  let comparisonRows=$derived(dialogMode==="io"&&doc.expected!==null?outputRows(doc):[]);
  let rawOutput=$state(false);
  let abnormal=$derived(doc.run.reason!=="exited"||doc.run.exit_code!=null&&doc.run.exit_code!==0);
  let warning=$derived(abnormal?(reasons[doc.run.reason]??doc.run.reason):doc.run.output.stdout_truncated?"输出截断":doc.run.cleanup_error?"清理异常":"");
  let groups=$derived([...new Set(list.map(m=>m.column.global?"全局 / 静态":m.column.fn??"局部变量"))]);
  let sourcePath=$state("");
  let fullLines=$derived(doc.run.session.sources.find(s=>s.path===sourcePath)?.text.split(/\r?\n/)??[]);
  const reasons:Record<string,string>={exited:"运行结束",nonzero_exit:"非零退出",signaled:"异常终止",compile_failed:"编译失败",limit_reached:"采集上限",timed_out:"超时",cancelled:"已取消",backend_failed:"采集失败",invalid_config:"配置错误"};
  function go(id:number,animate=true){
    if(!total||!Number.isFinite(id))return;
    const next=Math.max(0,Math.min(total-1,Math.round(id)));
    if(next===step)return;
    pulses=animate?pulseTargets(doc.run,step,next):new Set();
    motion={offset:animate?sourceMotion(doc.run,step,next):0,serial:motion.serial+1};
    step=next;
  }
  function choose(c:Column){selected=active===columnKey(c)?null:c;}
  function show(mode:"source"|"io"){
    rawOutput=false;dialogMode=mode;sourcePath=currentSource?.path??doc.run.session.sources[0]?.path??"";dialog.showModal();
  }
  function keys(e:KeyboardEvent){
    if(dialog?.open||e.metaKey||e.ctrlKey||e.altKey||["INPUT","SELECT","TEXTAREA"].includes((e.target as HTMLElement)?.tagName))return;
    const n=navigationTarget(e.key,e.shiftKey,step,total,events);
    if(n!==undefined){e.preventDefault();go(n);}
  }
  async function load(e:Event){
    const input=e.currentTarget as HTMLInputElement,file=input.files?.[0];if(!file)return;
    try{
      if(file.size>MAX_BYTES)throw Error("文件超过 128 MiB");
      const next=parseDocument(JSON.parse(await file.text()));
      outputText(next.run.output.stdout);outputText(next.run.output.stderr);
      eventCache=new Map();pulses=new Set();motion={offset:0,serial:motion.serial+1};doc=next;step=0;selected=null;error="";
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

</header>
{#if error}<div class="error" role="alert">{error}</div>{/if}
<section class="context">
  <button class="source-context" onclick={()=>show("source")} disabled={!doc.run.session.sources.length} aria-label="查看完整源码" title="查看完整源码">
    <div class="context-head"><span title={contextTitle}>{contextTitle}</span><span class="spacer"></span><span class="expand" aria-hidden="true">↗</span></div>
    <div class="context-lines">
      {#if line>0&&sourceLines[line-1]}
        <div class="source-window" use:slide={motion}>
          {#each context as r}<div class="code-line" class:current-line={r.line===line}><span class="line-no">{r.tokens?r.line:""}</span><code title={r.tokens?.map(t=>t.text).join("")}>{#each r.tokens??[] as t}<span class={t.kind}>{t.text}</span>{/each}</code></div>{/each}
        </div>
      {:else}<div class="code-line"><span class="line-no">—</span><code>无可映射的暂停源码</code></div>{/if}
    </div>
  </button>
  <button class="sample" onclick={()=>show("io")} title={warning||result} aria-label={"查看样例对比 · "+(warning||result)}>
    <span class="sample-title">{#if warning}<span class="bad">{warning}</span>{/if}<span class="spacer"></span><span class="expand" aria-hidden="true">↗</span></span>
    <span class="sample-row"><small>输入</small><code>{samplePreview(doc.stdin)}</code></span>
    <span class="sample-row"><small>预期</small><code>{samplePreview(doc.expected)}</code></span>
    <span class="sample-row" class:bad={result==="输出不一致"}><small title={result}>实际{result==="输出不一致"?" ≠":""}</small><code>{samplePreview(actual)}</code></span>
  </button>
</section>
<main>
  {#if current}
    {#if incomplete}<div class="capture-note">部分变量未记录</div>{/if}
    {#each groups as group}
      <section class="data-group">
        {#if groups.length>1}<h2>{group==="全局 / 静态"?group:group+" · 局部变量"}</h2>{/if}
        <div class="scalars">
          {#each list.filter(m=>(m.column.global?"全局 / 静态":m.column.fn??"局部变量")===group&&!isCompound(m.shape)) as m (columnKey(m.column))}
            {@const vs=selectedValues(current,m.column)}
            <div class="module-slot" style:--ideal={moduleWidth(m.shape)+"px"}><DataModule shape={m.shape} column={m.column} value={vs.length===1?vs[0]:undefined} previous={matchingPrevious(doc.run,current,m.column)} {active} onSelect={choose} {pulses} pulseSerial={motion.serial} root ambiguous={vs}/></div>
          {/each}
        </div>
        <div class="compounds">
          {#each list.filter(m=>(m.column.global?"全局 / 静态":m.column.fn??"局部变量")===group&&isCompound(m.shape)) as m (columnKey(m.column))}
            {@const vs=selectedValues(current,m.column)}
            <div class="module-slot" style:--ideal={moduleWidth(m.shape)+"px"}><DataModule shape={m.shape} column={m.column} value={vs.length===1?vs[0]:undefined} previous={matchingPrevious(doc.run,current,m.column)} {active} onSelect={choose} {pulses} pulseSerial={motion.serial} root ambiguous={vs}/></div>
          {/each}
        </div>
      </section>
    {/each}
    {#if !list.length}<div class="empty">没有已记录变量</div>{/if}
  {:else}<div class="empty">没有观察记录 · {reasons[doc.run.reason]??doc.run.reason}{#if doc.run.message}<p>{doc.run.message}</p>{/if}</div>{/if}
</main>
<Timeline {step} {total} {events} focus={selected?columnLabel(selected):""} onGo={go}/>
<dialog bind:this={dialog}>
  <div class="dialog-head"><strong>{dialogMode==="source"?"冻结源码":"输入输出"}</strong><span class="spacer"></span><button onclick={()=>dialog.close()} aria-label="关闭">×</button></div>
  <div class="dialog-body">
    {#if dialogMode==="source"}
      {#if doc.run.session.sources.length>1}<select aria-label="源码文件" bind:value={sourcePath}>{#each doc.run.session.sources as s}<option value={s.path}>{s.path}</option>{/each}</select>{/if}
      {#each fullLines as text,i}<div class="code-line" class:current-line={sourcePath===interval.origin?.file&&line===i+1}><span class="line-no">{i+1}</span><code>{text||" "}</code></div>{/each}
    {:else}
      <div class="io-head"><small>{result} · {doc.comparison||"原始字节"}</small><span class="spacer"></span>{#if doc.expected!==null}<button class="quiet" onclick={()=>rawOutput=!rawOutput}>{rawOutput?"逐行对比":"原始文本"}</button>{/if}</div>
      <h2>输入</h2><pre>{doc.stdin}</pre>
      {#if doc.expected!==null&&!rawOutput}
        <table class="output-table"><thead><tr><th>行</th><th>预期</th><th>实际 {doc.run.output.stdout_truncated?"· 截断":""}</th></tr></thead><tbody>{#each comparisonRows as r,i}<tr class:bad-row={!r.equal}><th>{i+1}</th><td>{r.expected??"—"}</td><td>{r.actual??"—"}</td></tr>{/each}</tbody></table>
        {#if !comparisonRows.length}<small>（均为空）</small>{/if}
        <small title="逐行表比较原始字节，样例结果按上方比较规则判定">· 空格 / ↵ 换行 / \r 回车 / \t 制表</small>
      {:else}
        <div class="outputs">{#if doc.expected!==null}<section><h2>预期</h2><pre>{doc.expected}</pre></section>{/if}<section><h2>实际 {doc.run.output.stdout_truncated?"· 截断":""}</h2><pre>{actual}</pre></section></div>
      {/if}
      {#if stderr||doc.run.output.stderr_truncated}<h2>stderr {doc.run.output.stderr_truncated?"· 截断":""}</h2><pre>{stderr}</pre>{/if}
      {#if abnormal||doc.run.message||doc.run.cleanup_error}<h2>运行状态</h2><p>{doc.run.reason} {doc.run.exit_code??""} {doc.run.message??""} {doc.run.cleanup_error??""}</p>{/if}
      {#each doc.run.diagnostics??[] as d}<p>{d.severity}: {d.message}</p>{/each}
    {/if}
  </div>
</dialog>
<style>
  :global(*){box-sizing:border-box}
  :global(body){margin:0;font:13px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif;color:var(--text);background:var(--bg);--bg:#f5f6f8;--panel:#fff;--text:#20242b;--muted:#626d7c;--line:#dfe3e9;--accent:#285cba;--soft:#eaf1ff;--hover:#f0f4fa;--mono:ui-monospace,SFMono-Regular,Consolas,monospace;color-scheme:light}
  @media(prefers-color-scheme:dark){:global(body){--bg:#161a20;--panel:#20262e;--text:#e5e9ef;--muted:#a1acb9;--line:#39434e;--accent:#92b8ff;--soft:#273a56;--hover:#29313b;color-scheme:dark}}
  :global(#app){height:100vh;display:flex;flex-direction:column;min-height:0}
  header,.context{flex:none;background:var(--panel)}
  header{display:flex;align-items:center;gap:10px;padding:7px 14px;border-bottom:1px solid var(--line);min-height:40px}
  header strong{font-size:15px}.spacer{flex:1}small{color:var(--muted);font-size:11px}
  button,select,.file{border:1px solid var(--line);border-radius:4px;padding:3px 8px;color:inherit;font:inherit;background:var(--panel);cursor:pointer}
  button:hover,.file:hover{background:var(--hover)}button:disabled{opacity:.35;cursor:default}
  .file input{position:absolute;opacity:0;width:1px;height:1px}.file:focus-within{outline:2px solid var(--accent)}
  .bad{color:#b45309}
  @media(prefers-color-scheme:dark){.bad{color:#f3ad71}}
  .context{display:grid;grid-template-columns:minmax(0,3fr) minmax(180px,2fr);gap:12px;padding:6px 14px 8px;border-bottom:1px solid var(--line)} .source-context{min-width:0;border:0;padding:0;background:transparent;text-align:left;overflow:hidden}.source-context:disabled{opacity:1;cursor:default}.source-context:hover:not(:disabled),.sample:hover{background:var(--hover)}.expand{font-size:12px;color:var(--muted);font-weight:400} .sample{display:grid;grid-template-rows:17px repeat(3,21px);min-width:0;text-align:left;border:0;border-left:1px solid var(--line);border-radius:0;padding:0 0 0 12px;overflow:hidden}.sample-title{display:flex;align-items:center;justify-content:space-between;color:var(--muted);font-size:11px}.sample-row{display:grid;grid-template-columns:42px minmax(0,1fr);gap:6px;align-items:center;min-width:0;min-height:0;height:21px;overflow:hidden}.sample-row code{display:block;min-width:0;max-width:100%;height:21px;overflow:hidden;text-overflow:ellipsis;white-space:pre;line-height:21px}.keyword{color:var(--accent);font-weight:600}.comment{color:var(--muted)}.literal{color:var(--muted)}
  .context-head{display:flex;align-items:center;color:var(--muted);font-size:11px;gap:8px;margin-bottom:3px}
  .context-head>span:first-child{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .quiet{border:0;padding:3px 7px;font-size:12px;background:transparent}
  .context-lines{height:63px;overflow:hidden;position:relative}
  .source-window{margin-top:-42px}
  .context-lines .code-line{height:21px;min-height:21px}
  .context-lines code{overflow:hidden;text-overflow:ellipsis;white-space:pre}
  .code-line{display:flex;gap:12px;min-height:21px;line-height:21px;align-items:baseline}
  .line-no{flex:0 0 30px;text-align:right;font:11px var(--mono);color:var(--muted)}
  code{font:12px/21px var(--mono);white-space:pre;min-width:0}
  .current-line{background:var(--soft);color:var(--accent);border-radius:3px}
  main{flex:1;min-height:0;overflow:auto;padding:12px 14px 18px}
  .data-group{margin-bottom:13px}h2{font-size:12px;margin:0 0 6px;color:var(--muted);font-weight:500}
  .scalars,.compounds{display:flex;flex-wrap:wrap;align-items:flex-start;gap:6px}
  .module-slot{width:min(100%,var(--ideal));max-width:100%;min-width:0}
  .compounds{gap:8px;margin-top:8px}.compounds .module-slot{flex:0 1 var(--ideal)}
  .capture-note{color:var(--muted);font-size:11px;margin-bottom:6px}
  .scalars:empty,.compounds:empty{display:none}
  .empty{color:var(--muted);padding:20px;text-align:center}
  button:focus-visible,select:focus-visible,input:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
  .io-head{display:flex;align-items:center;gap:8px}
  .error{padding:6px 14px;color:#b45309}
  dialog{color:var(--text);background:var(--panel);border:1px solid var(--line);border-radius:7px;padding:0;width:min(760px,90vw);max-height:85vh}
  dialog::backdrop{background:#0003}
  .dialog-head{display:flex;align-items:center;padding:8px 12px;border-bottom:1px solid var(--line)}
  .dialog-body{padding:12px;max-height:70vh;overflow:auto}
  .dialog-body h2{margin-top:12px}
  .outputs{display:grid;grid-template-columns:1fr 1fr;gap:10px}
  .output-table{border-collapse:collapse;width:100%;font:12px var(--mono);table-layout:fixed}.output-table th:first-child{width:40px}.output-table td,.output-table th{border:1px solid var(--line);padding:5px;text-align:left;white-space:pre-wrap;overflow-wrap:anywhere}.bad-row td{background:var(--soft);color:var(--accent)}
  @media(max-width:620px){.context{grid-template-columns:minmax(0,1fr) minmax(140px,.65fr);gap:6px}.sample{padding-left:6px}}
  pre{background:var(--bg);padding:8px;white-space:pre-wrap;overflow-wrap:anywhere}
</style>
