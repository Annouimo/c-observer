<script lang="ts">
  import {onMount} from "svelte";
  import brand from "../../../assets/icons/observer.svg?url";
  import spriteSource from "../ui/icons/sprite.svg?raw";
  const sprite=spriteSource.replace('<svg xmlns="http://www.w3.org/2000/svg">','<svg xmlns="http://www.w3.org/2000/svg" hidden aria-hidden="true">').replace(/<symbol id="([^"]+)"/g,(_,name)=>'<symbol id="routilet-icon-'+name+'"');
  import {fly} from "svelte/transition";
  import {connect,call,type Initial} from "./native";
  import example from "../../../testdata/snapshots/wrong-index.json";
  import DataModule from "./DataModule.svelte";
  import Timeline from "./Timeline.svelte";
  import {parseDocument,modules,selectedValues,matchingPrevious,changeEvents,navigationTarget,replayFeedback,columnKey,columnLabel,outputText,MAX_BYTES,visibleModules,isInline,moduleWidth,detailRoot,replayContext,sampleResult,outputRows,sourceTokens,samplePreview,type Column,type NavigationMode} from "./observation";
  let doc=$state(parseDocument(example));
  let screen=$state<"starting"|"setup"|"running"|"replay">("starting");
  let native=$state(false);
  let live=$state(false);
  let sessionMode=$state(false);
  let source=$state("");
  let stdin=$state("");
  let expected=$state("");
  let withExpected=$state(false);
  let cancelling=$state(false);
  let saving=$state(false);
  let notice=$state("");
  let previousScreen:"setup"|"replay"="setup";
  function failure(e:unknown){error=e instanceof Error?e.message:String(e);}
  function applyRecord(text:string,isLive:boolean){
    const next=parseDocument(JSON.parse(text));
    outputText(next.run.output.stdout);outputText(next.run.output.stderr);
    eventCache=new Map();lastNavigation=-Infinity;pulses=new Set();motion={offset:0,duration:135,serial:motion.serial+1};
    doc=next;step=0;selected=null;error="";notice="";live=isLive;screen="replay";
  }
  async function execute(method:"Run"|"RunSession"|"Rerun",...args:unknown[]){
    if(screen==="running")return;
    previousScreen=screen==="replay"?"replay":"setup";
    screen="running";error="";notice="";cancelling=false;
    try{applyRecord(await call<string>(method,...args),true);}
    catch(e){failure(e);screen=previousScreen;}
    finally{cancelling=false;}
  }
  async function cancelRun(){
    if(cancelling)return;
    cancelling=true;
    try{await call<void>("Cancel");}catch(e){failure(e);cancelling=false;}
  }
  async function pickSource(){
    try{const p=await call<string>("ChooseSource");if(p)source=p;}catch(e){failure(e);}
  }
  async function openNative(){
    try{const text=await call<string>("OpenRecord");if(text){applyRecord(text,false);sessionMode=false;}}catch(e){failure(e);}
  }
  async function exportRecord(){
    if(saving)return;saving=true;
    try{notice="已保存："+await call<string>("Export");}catch(e){failure(e);}
    finally{saving=false;}
  }
  function setup(){screen="setup";sessionMode=false;error="";notice="";}
  onMount(async()=>{
    native=await connect();
    if(!native){
      // Only Vite development may silently enter the synthetic example.
      if(import.meta.env.DEV){screen="replay";return;}
      error="本地应用通信初始化失败。";screen="setup";return;
    }
    try{
      const init=JSON.parse(await call<string>("Initial")) as Initial;
      source=init.Source??"";error=init.Error??"";screen="setup";
      if(init.Mode==="session"){sessionMode=true;await execute("RunSession");}
      else if(init.Mode==="replay")applyRecord(init.Record,false);
    }catch(e){failure(e);screen="setup";}
  });
  let step=$state(0);
  let motion=$state({offset:0,duration:135,serial:0});
  let lastNavigation=-Infinity;
  let pulses=$state<Set<string>>(new Set());
  // Only the small source strip moves. No animation queue and no delayed
  // snapshot state; cancel on new input and respect the OS motion preference.
  function slide(node:HTMLElement,initial:{offset:number;duration:number;serial:number}){
    let animation:Animation|undefined;
    return {update(next:{offset:number;duration:number;serial:number}){
      animation?.cancel();
      if(next.offset&&!matchMedia("(prefers-reduced-motion: reduce)").matches&&typeof node.animate==="function"){
        animation=node.animate([{transform:"translateY("+next.offset+"px)"},{transform:"translateY(0)"}],{duration:next.duration,easing:"ease-out"});
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
  let detail=$derived(detailRoot(allModules,selected));
  let detailKey=$derived(detail?columnKey(detail.column):"");
  function resetDetailScroll(node:HTMLElement,initial:string){return {update(next:string){node.scrollTop=0;node.scrollLeft=0;}};}
  let detailValues=$derived(detail&&current?selectedValues(current,detail.column):[]);
  let detailPrevious=$derived(detail&&current?matchingPrevious(doc.run,current,detail.column):undefined);
  const detailMotion=()=>({x:10,duration:matchMedia("(prefers-reduced-motion: reduce)").matches?0:140,opacity:0.6});
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
  function go(id:number,mode:NavigationMode="direct"){
    if(!total||!Number.isFinite(id))return;
    const next=Math.max(0,Math.min(total-1,Math.round(id)));
    if(next===step)return;
    const now=performance.now(),feedback=replayFeedback(doc.run,step,next,mode,now-lastNavigation);
    lastNavigation=now;
    pulses=feedback.pulses;
    motion={offset:feedback.offset,duration:feedback.duration,serial:motion.serial+1};
    step=next;
  }
  function choose(c:Column){selected=active===columnKey(c)?null:c;}
  function clearBlank(e:MouseEvent){const t=e.target as HTMLElement;if(!t.closest("button,.module-slot,aside,input,select,textarea")&&!window.getSelection()?.toString())selected=null;}
  function show(mode:"source"|"io"){
    rawOutput=false;dialogMode=mode;sourcePath=currentSource?.path??doc.run.session.sources[0]?.path??"";dialog.showModal();
  }
  function keys(e:KeyboardEvent){
    if(screen!=="replay"||dialog?.open||e.metaKey||e.ctrlKey||e.altKey||["INPUT","SELECT","TEXTAREA"].includes((e.target as HTMLElement)?.tagName))return;
    if(e.key==="Escape"){selected=null;return;}
    const n=navigationTarget(e.key,e.shiftKey,step,total,events);
    if(n!==undefined){e.preventDefault();go(n);}
  }
  async function load(e:Event){
    const input=e.currentTarget as HTMLInputElement,file=input.files?.[0];if(!file)return;
    try{
      if(file.size>MAX_BYTES)throw Error("文件超过 128 MiB");
      applyRecord(await file.text(),false);
    }catch(e){error=e instanceof Error?e.message:String(e);}
    input.value="";
  }
</script>
<svelte:window onkeydown={keys}/>
<!-- Trusted, vendored SVG resource, never user or record content. -->
{@html sprite}
<header>
  <div class="brand"><img src={brand} alt="" width="27" height="27"/><strong>C Observer</strong></div>
  {#if screen==="replay"}
    {#if native}
      {#if !sessionMode}<button class="ui-button" onclick={setup}><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-plus"/></svg>新建观察</button>{/if}
      {#if live}<button class="ui-button" onclick={()=>execute("Rerun")}><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-refresh-cw"/></svg>重跑</button><button class="ui-button" disabled={saving} onclick={exportRecord}>保存记录</button>{/if}
      {#if !sessionMode}<button class="ui-button" onclick={openNative}><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-folder-open"/></svg>打开记录</button>{/if}
    {:else}<label class="file">打开记录<input type="file" accept=".json,application/json" onchange={load}/></label>{/if}
  {/if}
  <span class="spacer"></span>
  {#if screen==="replay"}{#if doc.synthetic}<small>合成记录</small>{:else if !live}<small>离线记录</small>{/if}{/if}

</header>
{#if error}<div class="error" role="alert">{error}</div>{/if}
{#if notice}<div class="notice" role="status">{notice}</div>{/if}
{#if screen==="starting"}
  <main class="center"><span>正在启动…</span></main>
{:else if screen==="running"}
  <main class="center"><section class="run-card"><span class="spinner" aria-hidden="true"></span><h1>{cancelling?"正在停止并清理":"正在编译和采集"}</h1><p>{source.split(/[\\/]/).at(-1)}</p><button class="ui-button" disabled={cancelling} onclick={cancelRun}>取消</button></section></main>
{:else if screen==="setup"}
  <main class="center">
    <section class="setup-card">
      <h1>新建观察</h1>
      <label for="source-path">C 源文件</label>
      <div class="source-picker"><input class="ui-field" id="source-path" bind:value={source} placeholder="选择或输入 .c 文件路径"/><button class="ui-button" data-kind="outline" disabled={!native} onclick={pickSource}><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-folder-open"/></svg>选择文件</button></div>
      <div class="samples">
        <label>输入<textarea class="ui-textarea" bind:value={stdin} spellcheck={false} placeholder="可为空"></textarea></label>
        <label><span class="expected-label"><input type="checkbox" bind:checked={withExpected}/> 预期输出</span><textarea class="ui-textarea" bind:value={expected} disabled={!withExpected} spellcheck={false} placeholder="可选"></textarea></label>
      </div>
      <button class="ui-button" data-kind="primary" disabled={!native||!source.trim()} onclick={()=>execute("Run",source,stdin,withExpected,expected)}><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-play"/></svg>运行观察</button>
      <button class="ui-button quiet open-record" disabled={!native} onclick={openNative}>打开已有观察记录</button>
    </section>
  </main>
{:else}
<section class="context">
  <button class="source-context" onclick={()=>show("source")} disabled={!doc.run.session.sources.length} aria-label="查看完整源码" title="查看完整源码">
    <div class="context-head"><span title={contextTitle}>{contextTitle}</span><span class="spacer"></span><span class="expand" aria-hidden="true"><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-external-link"/></svg></span></div>
    <div class="context-lines">
      {#if line>0&&sourceLines[line-1]}
        <div class="source-window" use:slide={motion}>
          {#each context as r}<div class="code-line" class:current-line={r.line===line}><span class="line-no">{r.tokens?r.line:""}</span><code title={r.tokens?.map(t=>t.text).join("")}>{#each r.tokens??[] as t}<span class={t.kind}>{t.text}</span>{/each}</code></div>{/each}
        </div>
      {:else}<div class="code-line"><span class="line-no">—</span><code>无可映射的暂停源码</code></div>{/if}
    </div>
  </button>
  <button class="sample" onclick={()=>show("io")} title={warning||result} aria-label={"查看样例对比 · "+(warning||result)}>
    <span class="sample-title">{#if warning}<span class="bad">{warning}</span>{/if}<span class="spacer"></span><span class="expand" aria-hidden="true"><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-external-link"/></svg></span></span>
    <span class="sample-row"><small>输入</small><code>{samplePreview(doc.stdin)}</code></span>
    <span class="sample-row"><small>预期</small><code>{samplePreview(doc.expected)}</code></span>
    <span class="sample-row" class:bad={result==="输出不一致"}><small title={result}>实际{result==="输出不一致"?" ≠":""}</small><code>{samplePreview(actual)}</code></span>
  </button>
</section>
<div class="data-workspace">
<main class="data-main" onclick={clearBlank} role="presentation">
  {#if current}
    {#if incomplete}<div class="capture-note">部分变量未记录</div>{/if}
    {#each groups as group}
      <section class="data-group">
        {#if groups.length>1}<h2>{group==="全局 / 静态"?group:group+" · 局部变量"}</h2>{/if}
        <div class="scalars">
          {#each list.filter(m=>(m.column.global?"全局 / 静态":m.column.fn??"局部变量")===group&&isInline(m.shape)) as m (columnKey(m.column))}
            {@const vs=selectedValues(current,m.column)}
            <div class="module-slot" style:--ideal={moduleWidth(m.shape)+"px"}><DataModule shape={m.shape} column={m.column} value={vs.length===1?vs[0]:undefined} previous={matchingPrevious(doc.run,current,m.column)} {active} onSelect={choose} {pulses} pulseSerial={motion.serial} root ambiguous={vs}/></div>
          {/each}
        </div>
        <div class="compounds">
          {#each list.filter(m=>(m.column.global?"全局 / 静态":m.column.fn??"局部变量")===group&&!isInline(m.shape)) as m (columnKey(m.column))}
            {@const vs=selectedValues(current,m.column)}
            <div class="module-slot" style:--ideal={moduleWidth(m.shape)+"px"}><DataModule shape={m.shape} column={m.column} value={vs.length===1?vs[0]:undefined} previous={matchingPrevious(doc.run,current,m.column)} {active} onSelect={choose} {pulses} pulseSerial={motion.serial} root ambiguous={vs}/></div>
          {/each}
        </div>
      </section>
    {/each}
    {#if !list.length}<div class="empty">没有已记录变量</div>{/if}
  {:else}<div class="empty">没有观察记录 · {reasons[doc.run.reason]??doc.run.reason}{#if doc.run.message}<p>{doc.run.message}</p>{/if}</div>{/if}
</main>
{#if detail}<aside class="detail-pane" use:resetDetailScroll={detailKey} aria-label={"详情 · "+columnLabel(detail.column)} transition:fly={detailMotion()}>
  {#key detailKey}<DataModule shape={detail.shape} column={detail.column} value={detailValues.length===1?detailValues[0]:undefined} previous={detailPrevious} {active} onSelect={choose} {pulses} pulseSerial={motion.serial} root ambiguous={detailValues} full/>{/key}
</aside>{/if}
</div>
<Timeline {step} {total} {events} focus={selected?columnLabel(selected):""} onGo={go}/>
<dialog class="ui-dialog" bind:this={dialog}>
  <div class="dialog-head"><strong>{dialogMode==="source"?"冻结源码":"输入输出"}</strong><span class="spacer"></span><button class="ui-button ui-icon-button" onclick={()=>dialog.close()} aria-label="关闭"><svg class="ui-icon" aria-hidden="true"><use href="#routilet-icon-x"/></svg></button></div>
  <div class="dialog-body">
    {#if dialogMode==="source"}
      {#if doc.run.session.sources.length>1}<select class="ui-select" aria-label="源码文件" bind:value={sourcePath}>{#each doc.run.session.sources as s}<option value={s.path}>{s.path}</option>{/each}</select>{/if}
      {#each fullLines as text,i}<div class="code-line" class:current-line={sourcePath===interval.origin?.file&&line===i+1}><span class="line-no">{i+1}</span><code>{text||" "}</code></div>{/each}
    {:else}
      <div class="io-head"><small>{result} · {doc.comparison||"原始字节"}</small><span class="spacer"></span>{#if doc.expected!==null}<button class="ui-button quiet" onclick={()=>rawOutput=!rawOutput}>{rawOutput?"逐行对比":"原始文本"}</button>{/if}</div>
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
{/if}
<style>
  :global(body){--bg:var(--ui-canvas);--panel:var(--ui-surface);--text:var(--ui-text);--muted:var(--ui-muted);--line:var(--ui-line);--accent:var(--ui-accent);--soft:var(--ui-selected);--hover:var(--ui-hover);--mono:var(--ui-mono);color:var(--text);background:var(--bg)}
  :global(#app){height:100vh;display:flex;flex-direction:column;min-height:0}
  header,.context{flex:none;background:var(--panel)}
  header{display:flex;align-items:center;gap:8px;flex-wrap:wrap;padding:7px 14px;border-bottom:1px solid var(--line);min-height:44px}
  header strong{font-size:var(--ui-heading)}.brand{display:flex;align-items:center;gap:8px;flex:none}.brand img{display:block}.spacer{flex:1}small{color:var(--muted);font-size:11px}
  .file{border:1px solid var(--line);border-radius:var(--ui-radius);padding:5px 9px;color:inherit;font:inherit;background:var(--panel);cursor:pointer}
  .file:hover{background:var(--hover)}
  .file input{position:absolute;opacity:0;width:1px;height:1px}.file:focus-within{outline:var(--ui-focus-width) solid var(--ui-focus);outline-offset:-1px}
  .bad{color:var(--ui-danger)}
  .context{display:grid;grid-template-columns:minmax(0,5fr) minmax(180px,3fr);gap:12px;padding:6px 14px 8px;border-bottom:1px solid var(--line)} .source-context{min-width:0;border:0;padding:0;background:transparent;color:inherit;text-align:left;overflow:hidden}.source-context:disabled{opacity:1;cursor:default}.source-context:hover:not(:disabled),.sample:hover{background:var(--hover)}.expand{font-size:12px;color:var(--muted);font-weight:400} .sample{background:transparent;color:inherit;display:grid;grid-template-rows:17px repeat(3,21px);min-width:0;text-align:left;border:0;border-left:1px solid var(--line);border-radius:0;padding:0 0 0 12px;overflow:hidden}.sample-title{display:flex;align-items:center;justify-content:space-between;color:var(--muted);font-size:11px}.sample-row{display:grid;grid-template-columns:42px minmax(0,1fr);gap:6px;align-items:center;min-width:0;min-height:0;height:21px;overflow:hidden}.sample-row code{display:block;min-width:0;max-width:100%;height:21px;overflow:hidden;text-overflow:ellipsis;white-space:pre;line-height:21px}.keyword{color:var(--accent);font-weight:600}.comment{color:var(--muted)}.literal{color:var(--muted)}
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
  .data-workspace{display:flex;flex:1;min-height:0;overflow:hidden}
  .data-main{min-width:0}
  .detail-pane{flex:0 0 clamp(260px,36%,440px);min-width:0;overflow:auto;border-left:1px solid var(--line);padding:12px 10px;background:var(--panel)}
  @media(max-width:620px){.detail-pane{flex-basis:44%;min-width:180px;padding:8px}}
  .data-group{margin-bottom:13px}h2{font-size:12px;margin:0 0 6px;color:var(--muted);font-weight:500}
  .scalars,.compounds{display:flex;flex-wrap:wrap;align-items:flex-start;gap:6px}
  .module-slot{width:min(100%,var(--ideal));max-width:100%;min-width:0}
  .scalars .module-slot{width:max-content;min-width:min(100%,var(--ideal));max-width:100%}
  .compounds{gap:8px;margin-top:8px}.compounds .module-slot{flex:0 0 auto;width:min(100%,var(--ideal))}
  .capture-note{color:var(--muted);font-size:11px;margin-bottom:6px}
  .scalars:empty,.compounds:empty{display:none}
  .empty{color:var(--muted);padding:20px;text-align:center}
  .io-head{display:flex;align-items:center;gap:8px}
  .error{padding:6px 14px;color:var(--ui-danger);overflow-wrap:anywhere}
  .notice{padding:6px 14px;color:var(--muted);overflow-wrap:anywhere}
  .center{display:flex;align-items:center;justify-content:center}
  .setup-card{width:min(620px,100%);display:grid;gap:10px;padding:20px}
  .setup-card h1,.run-card h1{font-size:18px;margin:0 0 6px;font-weight:600}
  .setup-card label{font-size:12px;color:var(--muted)}
  .source-picker{display:flex;gap:6px}
  .source-picker input,textarea{min-width:0;width:100%;font:13px var(--mono)}
  .source-picker button{flex:none}.samples{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}
  .samples label{display:flex;flex-direction:column;gap:5px}.expected-label{display:flex;align-items:center;height:17px;gap:4px}
  textarea{height:145px;resize:vertical}
  .open-record{justify-self:center;color:var(--muted)}
  .run-card{display:grid;justify-items:center;gap:10px;text-align:center}
  .run-card p{color:var(--muted);margin:0}
  .spinner{width:22px;height:22px;border:2px solid var(--line);border-top-color:var(--accent);border-radius:50%;animation:spin .8s linear infinite}
  @keyframes spin{to{transform:rotate(360deg)}}@media(prefers-reduced-motion:reduce){.spinner{animation:none}}
  dialog.ui-dialog{padding:0;width:min(760px,90vw);max-height:85vh}
  
  .dialog-head{display:flex;align-items:center;padding:8px 12px;border-bottom:1px solid var(--line)}
  .dialog-body{padding:12px;max-height:70vh;overflow:auto}
  .dialog-body h2{margin-top:12px}
  .outputs{display:grid;grid-template-columns:1fr 1fr;gap:10px}
  .output-table{border-collapse:collapse;width:100%;font:12px var(--mono);table-layout:fixed}.output-table th:first-child{width:40px}.output-table td,.output-table th{border:1px solid var(--line);padding:5px;text-align:left;white-space:pre-wrap;overflow-wrap:anywhere}.bad-row td{background:var(--soft);color:var(--accent)}
  @media(max-width:620px){.context{grid-template-columns:minmax(0,1fr) minmax(140px,.65fr);gap:6px}.sample{padding-left:6px}}
  pre{background:var(--bg);padding:8px;white-space:pre-wrap;overflow-wrap:anywhere}
</style>
