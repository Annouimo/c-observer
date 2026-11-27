<script lang="ts">
  import Self from "./DataModule.svelte";
  import {columnKey,columnLabel,member,memberLabel,childValue,observedChange,displayValue,sharedFields,label,arraySegments,foldValue,moduleWidth,coverageNote,statusHint,compactValue,stringSummary,hasDetail,previewChildren,type Shape,type Variable,type Column} from "./observation";
  let {shape,column,value,previous,active,onSelect,root=false,ambiguous=[],pulses=new Set<string>(),pulseSerial=0,full=false}:{
    shape:Shape;column:Column;value?:Variable;previous?:Variable;active:string;
    onSelect:(c:Column)=>void;root?:boolean;ambiguous?:Variable[];pulses?:Set<string>;pulseSerial?:number;full?:boolean;
  }=$props();
  let chosen=$derived(active===columnKey(column));
  let diff=$derived(observedChange(previous,value));
  let summary=$derived(stringSummary(value));
  let shown=$derived(full?shape.children:hasDetail(shape)?previewChildren(shape):shape.children);
  let compound=$derived(shape.children.length>0||["array","struct","union"].includes(shape.kind));
  let fields=$derived(shape.kind==="array"?sharedFields({...shape,children:shown}):[]);
  let tabular=$derived(fields.length>0&&shape.children.every(c=>["array","struct","unknown"].includes(c.kind))&&fields.every(c=>!c.children.length||!full&&c.kind==="string"));
  let segments=$derived(shape.kind==="array"?(full?shown.map(c=>[c]):arraySegments({...shape,children:shown})):[]);
  function flash(node:HTMLElement,initial:number){
    let a:Animation|undefined;
    function update(serial:number){
      a?.cancel();
      if(serial&&node.animate&&!matchMedia("(prefers-reduced-motion: reduce)").matches){
        // Explicit RGBA avoids custom-property interpolation differences
        // in the platform WebView. Do not animate the value itself.
        a=node.animate([{boxShadow:"inset 0 0 0 100px rgba(57,120,208,.28)"},{boxShadow:"inset 0 0 0 100px rgba(57,120,208,0)"}],{duration:200,easing:"ease-out"});
      }
    }
    update(initial);
    return {update,destroy(){a?.cancel();}};
  }
  function pulse(c:Column){return pulses.has(columnKey(c))?pulseSerial:0;}
  let coverage=$derived(coverageNote(shape,value,root));
  let title=$derived(root?column.name:memberLabel(shape).replace(/^\./,""));
</script>
{#if ambiguous.length>1}
  <div class="module ambiguous"><strong>{title}</strong><small>同名 {ambiguous.length} 项，无法唯一对应</small>{#each ambiguous as item}<small>{item.type} · {compactValue(item)}</small>{/each}</div>
{:else if !full&&shape.kind==="string"}
  <button use:flash={pulse(column)} class="module scalar string-summary" class:chosen class:diff aria-pressed={chosen} onclick={()=>onSelect(column)} title={columnLabel(column)+" · "+statusHint(value,root)}><span class="scalar-name">{title}{shape.arrayLength!==undefined?"["+shape.arrayLength+"]":""} =</span><span class="number">{compactValue(value,root)}</span></button>
{:else if compound}
  <section class="module compound" class:full class:chosen class:root class:absent={!value&&root}>
    <button class="heading" aria-pressed={chosen} onclick={()=>onSelect(column)} title={columnLabel(column)+" · "+(value?value.type+" · "+label(value):displayValue(value,root))}>
      <span>{title}{(shape.kind==="array"||shape.kind==="string")&&shape.arrayLength!==undefined?"["+shape.arrayLength+"]":""}</span>
      {#if shape.kind==="pointer"}<small class="address" title={compactValue(value)}>{compactValue(value)}</small>{:else if coverage}<small title="本次记录覆盖情况">{coverage}</small>{/if}
      {#if diff&&(!shape.children.length||shape.children.length>8)}<span class="dot" title="与前一观察有已知差异"></span>{/if}
    </button>
    {#if full&&shape.kind==="string"}
      {#if summary}<div class="string-value">{summary.text}</div>{/if}
      <div class="array inner-scroll">{#each shown as ch}{@const c=member(column,ch)}{@const v=childValue(value,ch)}{@const p=childValue(previous,ch)}<button use:flash={pulse(c)} class="element character" class:tail={summary?.end!==undefined&&ch.index!==undefined&&ch.index>summary.end} class:terminator={summary?.end!==undefined&&ch.index===summary.end} class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+statusHint(v)}><small>{ch.index??ch.name}</small><span>{displayValue(v)}</span></button>{/each}</div>
    {:else if tabular}
      <div class="inner-scroll"><table><colgroup><col style:width="38px"/>{#each fields as field}<col style:width={Math.max(45,8*Math.max(field.name.length,field.width??1)+16)+"px"}/>{/each}</colgroup><thead><tr><th aria-label="下标"></th>{#each fields as field}<th>{memberLabel(field).replace(/^\./,"")}</th>{/each}</tr></thead>
        <tbody>{#each shown as row}
          {@const rv=childValue(value,row)}{@const rp=childValue(previous,row)}{@const rc=member(column,row)}
          <tr><th><button class="index" class:chosen={active===columnKey(rc)} aria-pressed={active===columnKey(rc)} onclick={()=>onSelect(rc)}>{row.index??row.name}</button></th>
            {#each fields as field}
              {@const v=childValue(rv,field)}{@const p=childValue(rp,field)}{@const c=member(rc,field)}
              <td><button use:flash={pulse(c)} class="cell" class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} class:unknown={!v||v.status!=="observed"} aria-pressed={active===columnKey(c)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+statusHint(v)+(observedChange(p,v)?"（之前 "+displayValue(p)+"）":"")}>{compactValue(v)}</button></td>
            {/each}
          </tr>
        {/each}</tbody>
      </table></div>
    {:else if shape.kind==="array"&&shape.children.every(c=>!c.children.length&&!["array","struct","union"].includes(c.kind))}
      <div class="array inner-scroll">
        {#each segments as segment,i}
          {@const folded=foldValue(segment,value,previous)}
          {#if folded!==undefined}
            <button class="range" data-range={i} onclick={()=>onSelect(column)} title="关注数组并查看完整已记录元素"><small>{segment[0].index}–{segment.at(-1)?.index}</small><span>{folded} × {segment.length} ›</span></button>
          {:else}
            {#each segment as ch}
              {@const c=member(column,ch)}{@const v=childValue(value,ch)}{@const p=childValue(previous,ch)}
              <button use:flash={pulse(c)} data-range-item={segment.length>1?i:undefined} class="element" style:width={Math.max(42,Math.min(350,8*Math.max(ch.width??1,String(ch.index??ch.name).length)+16))+"px"} class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} class:unknown={!v||v.status!=="observed"} aria-pressed={active===columnKey(c)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+statusHint(v)+(observedChange(p,v)?"（之前 "+displayValue(p)+"）":"")}>
                <small>{ch.index??ch.name}</small><span>{displayValue(v)}</span>
              </button>
            {/each}
          {/if}
        {/each}
      </div>
    {:else}
      <div class="members inner-scroll">
        {#each shown as ch}
          <div class="nested" style:--ideal={moduleWidth(ch)+"px"}><Self shape={ch} column={member(column,ch)} value={childValue(value,ch)} previous={childValue(previous,ch)} {active} {onSelect} {pulses} {pulseSerial} {full}/></div>
        {/each}
        {#if !shape.children.length}<small>没有已记录成员</small>{/if}
      </div>
    {/if}
    {#if !full&&shown.length<shape.children.length}<button class="more" onclick={()=>onSelect(column)} title="关注并查看全部已记录数据">… {shown.length}/{shape.children.length}</button>{/if}
  </section>
{:else}
  <button use:flash={pulse(column)} class="module scalar" class:chosen class:diff class:root class:unknown={!value||value.status!=="observed"} aria-pressed={chosen} onclick={()=>onSelect(column)} title={columnLabel(column)+" · "+shape.type+": "+statusHint(value,root)+(diff?"（之前 "+displayValue(previous)+"）":"")}>
    <span class="scalar-name">{title} =</span><span class="number">{compactValue(value,root)}</span>
  </button>
{/if}
<style>
  .module{border:1px solid var(--line);border-radius:5px;background:var(--panel);min-width:0;color:var(--text)}
  .chosen,.module.chosen{outline:2px solid var(--accent);outline-offset:-2px}
  button{font:inherit;color:inherit;cursor:pointer;border:0;background:transparent}
  button:hover{background:var(--hover)}button:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}
  .scalar{display:flex;align-items:baseline;gap:5px;text-align:left;padding:5px 8px;width:100%;min-width:0;min-height:30px;position:relative}
  .scalar-name{font:12px var(--mono);color:var(--muted);flex:0 0 auto;min-width:0;max-width:55%;overflow-wrap:anywhere;white-space:normal}
  .number{font:600 14px/1.35 var(--mono);flex:0 1 auto;min-width:0;overflow-wrap:anywhere;white-space:normal}
  .scalar.diff .number{font-weight:700}
  .diff{color:var(--accent);background:var(--soft)}
  .unknown .number,.unknown.cell,.unknown.element{color:var(--muted)}
  small{color:var(--muted);font-size:11px}
  .heading{display:flex;align-items:center;gap:8px;width:100%;text-align:left;padding:6px 9px;border-bottom:1px solid var(--line);font:600 13px var(--mono);min-height:32px}
  .heading>span:first-child{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .heading small{font:11px system-ui;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-left:auto}
  .dot{background:var(--accent);height:6px;width:6px;border-radius:100%;flex:none;margin-left:auto}
  .heading small+.dot{margin-left:0}
  .inner-scroll{max-height:290px;overflow:auto;min-width:0}
  .array{display:flex;flex-wrap:wrap;align-content:start;padding:5px;gap:3px}
  .element{display:flex;flex-direction:column;align-items:center;justify-content:center;border:1px solid var(--line);border-radius:3px;min-height:38px;padding:3px;min-width:0;max-width:100%;font:13px var(--mono)}
  .element span{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  table{border-collapse:separate;border-spacing:0;width:100%;table-layout:fixed;font:13px var(--mono)}
  th{font-weight:400;color:var(--muted);background:var(--panel);white-space:nowrap;font-size:11px}
  thead th{position:sticky;top:0;z-index:1;padding:5px 8px;border-bottom:1px solid var(--line)}
  td,tbody th{border-bottom:1px solid var(--line);border-right:1px solid var(--line);padding:0;text-align:center}
  td:last-child{border-right:0}
  tbody tr:last-child td,tbody tr:last-child th{border-bottom:0}
  .cell,.index{display:block;width:100%;min-width:58px;min-height:29px;padding:4px 8px;font-family:var(--mono);white-space:nowrap}
  .index{min-width:32px;color:var(--muted)}
  .members{display:flex;flex-wrap:wrap;gap:5px;padding:5px;align-items:start}
  .nested{width:min(100%,var(--ideal));min-width:0}
  .range{display:flex;flex-direction:column;justify-content:center;align-items:center;min-height:38px;min-width:126px;border:1px dashed var(--line);border-radius:3px;padding:3px 7px;font:12px var(--mono);color:var(--muted)}
  .range-close{font:11px system-ui;color:var(--muted);border:1px dashed var(--line);padding:3px 7px}
  .cell{overflow:hidden;text-overflow:ellipsis}
  .diff.cell,.diff.element{font-weight:600}
  .full .inner-scroll{max-height:none;overflow:visible}
  .full .nested{max-width:100%}
  .full .cell,.full .element span{white-space:normal;overflow-wrap:anywhere;overflow:visible}
  .full table{table-layout:auto}
  .string-value{padding:7px 9px;font:600 14px var(--mono);overflow-wrap:anywhere}
  .character{min-width:48px;padding:4px 6px}.character.tail:not(.diff){color:var(--muted)}.character.terminator{border-style:dashed}
  .more{display:block;width:100%;text-align:right;font:11px var(--mono);color:var(--muted);padding:4px 8px}
  .address{max-width:70%;font-family:var(--mono)!important}
  .ambiguous{padding:8px}.ambiguous small{display:block}.ambiguous pre{max-height:200px;overflow:auto;white-space:pre-wrap}
</style>
