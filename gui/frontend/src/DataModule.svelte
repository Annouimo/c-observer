<script lang="ts">
  import {tick} from "svelte";
  import Self from "./DataModule.svelte";
  import {columnKey,columnLabel,member,memberLabel,childValue,observedChange,displayValue,sharedFields,label,arraySegments,foldValue,moduleWidth,coverageNote,statusHint,type Shape,type Variable,type Column} from "./observation";
  let {shape,column,value,previous,active,onSelect,root=false,ambiguous=[],pulses=new Set<string>(),pulseSerial=0}:{
    shape:Shape;column:Column;value?:Variable;previous?:Variable;active:string;
    onSelect:(c:Column)=>void;root?:boolean;ambiguous?:Variable[];pulses?:Set<string>;pulseSerial?:number;
  }=$props();
  let chosen=$derived(active===columnKey(column));
  let diff=$derived(observedChange(previous,value));
  let compound=$derived(shape.children.length>0||["array","struct","union"].includes(shape.kind));
  let fields=$derived(shape.kind==="array"?sharedFields(shape):[]);
  let tabular=$derived(fields.length>0&&shape.children.every(c=>["array","struct","unknown"].includes(c.kind))&&fields.every(c=>!c.children.length));
  let segments=$derived(shape.kind==="array"?arraySegments(shape):[]);
  let expanded=$state<number[]>([]);
  let arrayNode:HTMLDivElement;
  let rangeMotion:Animation|undefined;
  async function toggleRange(i:number){
    expanded=expanded.includes(i)?expanded.filter(x=>x!==i):[...expanded,i];
    rangeMotion?.cancel();
    await tick();
    if(matchMedia("(prefers-reduced-motion: reduce)").matches)return;
    const el=arrayNode?.querySelector<HTMLElement>('[data-range="'+i+'"]');
    if(el?.animate)rangeMotion=el.animate([{opacity:.3,transform:"translateY(-3px)"},{opacity:1,transform:"translateY(0)"}],{duration:150,easing:"ease-out"});
  }
  function flash(node:HTMLElement,initial:number){
    let a:Animation|undefined;
    return {update(serial:number){
      a?.cancel();
      if(serial&&node.animate&&!matchMedia("(prefers-reduced-motion: reduce)").matches){
        a=node.animate([{boxShadow:"inset 0 0 0 100px var(--flash)"},{boxShadow:"inset 0 0 0 100px transparent"}],{duration:180,easing:"ease-out"});
      }
    },destroy(){a?.cancel();}};
  }
  function pulse(c:Column){return pulses.has(columnKey(c))?pulseSerial:0;}
  let coverage=$derived(coverageNote(shape,value,root));
  let title=$derived(root?column.name:memberLabel(shape).replace(/^\./,""));
</script>
{#if ambiguous.length>1}
  <div class="module ambiguous"><strong>{title}</strong><small>同名 {ambiguous.length} 项，无法唯一对应</small><details><summary>已记录数据</summary><pre>{JSON.stringify(ambiguous,null,2)}</pre></details></div>
{:else if compound}
  <section class="module compound" class:chosen class:root class:absent={!value&&root}>
    <button class="heading" aria-pressed={chosen} onclick={()=>onSelect(column)} title={columnLabel(column)+" · "+(value?value.type+" · "+label(value):displayValue(value,root))}>
      <span>{title}{shape.kind==="array"&&shape.arrayLength!==undefined?"["+shape.arrayLength+"]":""}</span>
      {#if coverage}<small title="本次记录覆盖情况">{coverage}</small>{/if}
      {#if diff&&(!shape.children.length||shape.children.length>8)}<span class="dot" title="与前一观察有已知差异"></span>{/if}
    </button>
    {#if tabular}
      <div class="inner-scroll"><table><colgroup><col style:width="38px"/>{#each fields as field}<col style:width={Math.max(45,8*Math.max(field.name.length,field.width??1)+16)+"px"}/>{/each}</colgroup><thead><tr><th aria-label="下标"></th>{#each fields as field}<th>{memberLabel(field).replace(/^\./,"")}</th>{/each}</tr></thead>
        <tbody>{#each shape.children as row}
          {@const rv=childValue(value,row)}{@const rp=childValue(previous,row)}{@const rc=member(column,row)}
          <tr><th><button class="index" class:chosen={active===columnKey(rc)} aria-pressed={active===columnKey(rc)} onclick={()=>onSelect(rc)}>{row.index??row.name}</button></th>
            {#each fields as field}
              {@const v=childValue(rv,field)}{@const p=childValue(rp,field)}{@const c=member(rc,field)}
              <td><button use:flash={pulse(c)} class="cell" class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} class:unknown={!v||v.status!=="observed"} aria-pressed={active===columnKey(c)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+statusHint(v)+(observedChange(p,v)?"（之前 "+displayValue(p)+"）":"")}>{displayValue(v)}</button></td>
            {/each}
          </tr>
        {/each}</tbody>
      </table></div>
    {:else if shape.kind==="array"&&shape.children.every(c=>!c.children.length&&!["array","struct","union"].includes(c.kind))}
      <div class="array inner-scroll" bind:this={arrayNode}>
        {#each segments as segment,i}
          {@const folded=foldValue(segment,value,previous)}
          {#if folded!==undefined&&!expanded.includes(i)}
            <button class="range" data-range={i} onclick={()=>toggleRange(i)} title="展开全部已记录元素"><small>{segment[0].index}–{segment.at(-1)?.index}</small><span>{folded} × {segment.length} ›</span></button>
          {:else}
            {#if expanded.includes(i)&&folded!==undefined}<button class="range-close" data-range={i} onclick={()=>toggleRange(i)}>收起 {segment[0].index}–{segment.at(-1)?.index}</button>{/if}
            {#each segment as ch}
              {@const c=member(column,ch)}{@const v=childValue(value,ch)}{@const p=childValue(previous,ch)}
              <button use:flash={pulse(c)} class="element" style:width={Math.max(42,Math.min(350,8*Math.max(ch.width??1,String(ch.index??ch.name).length)+16))+"px"} class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} class:unknown={!v||v.status!=="observed"} aria-pressed={active===columnKey(c)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+statusHint(v)+(observedChange(p,v)?"（之前 "+displayValue(p)+"）":"")}>
                <small>{ch.index??ch.name}</small><span>{displayValue(v)}</span>
              </button>
            {/each}
          {/if}
        {/each}
      </div>
    {:else}
      <div class="members inner-scroll">
        {#each shape.children as ch}
          <div class="nested" style:--ideal={moduleWidth(ch)+"px"}><Self shape={ch} column={member(column,ch)} value={childValue(value,ch)} previous={childValue(previous,ch)} {active} {onSelect} {pulses} {pulseSerial}/></div>
        {/each}
        {#if !shape.children.length}<small>没有已记录成员</small>{/if}
      </div>
    {/if}
  </section>
{:else}
  <button use:flash={pulse(column)} class="module scalar" class:chosen class:diff class:root class:unknown={!value||value.status!=="observed"} aria-pressed={chosen} onclick={()=>onSelect(column)} title={columnLabel(column)+" · "+shape.type+": "+statusHint(value,root)+(diff?"（之前 "+displayValue(previous)+"）":"")}>
    <span class="scalar-name">{title} =</span><span class="number">{displayValue(value,root)}</span>
  </button>
{/if}
<style>
  .module{border:1px solid var(--line);border-radius:5px;background:var(--panel);min-width:0;color:var(--text)}
  .chosen,.module.chosen{outline:2px solid var(--accent);outline-offset:-2px}
  button{font:inherit;color:inherit;cursor:pointer;border:0;background:transparent}
  button:hover{background:var(--hover)}button:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}
  .scalar{display:flex;align-items:baseline;gap:5px;text-align:left;padding:5px 8px;width:100%;min-width:0;min-height:30px;position:relative}
  .scalar-name{font:12px var(--mono);color:var(--muted);flex:0 1 auto;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .number{font:600 14px/1.35 var(--mono);flex:0 0 auto;max-width:65%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0}
  .scalar.diff .number{font-weight:700}
  .diff{color:var(--accent);background:var(--soft)}
  button{--flash:rgba(57,120,208,.20)}
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
  .ambiguous{padding:8px}.ambiguous small{display:block}.ambiguous pre{max-height:200px;overflow:auto;white-space:pre-wrap}
</style>
