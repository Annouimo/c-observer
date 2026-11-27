<script lang="ts">
  import Self from "./DataModule.svelte";
  import {columnKey,columnLabel,member,memberLabel,childValue,observedChange,displayValue,sharedFields,label,type Shape,type Variable,type Column} from "./observation";
  let {shape,column,value,previous,active,onSelect,root=false,ambiguous=[]}:{
    shape:Shape;column:Column;value?:Variable;previous?:Variable;active:string;
    onSelect:(c:Column)=>void;root?:boolean;ambiguous?:Variable[];
  }=$props();
  let chosen=$derived(active===columnKey(column));
  let diff=$derived(observedChange(previous,value));
  let compound=$derived(shape.children.length>0||["array","struct","union"].includes(shape.kind));
  let fields=$derived(shape.kind==="array"?sharedFields(shape):[]);
  let tabular=$derived(fields.length>0&&shape.children.every(c=>["array","struct","unknown"].includes(c.kind))&&fields.every(c=>!c.children.length));
  let title=$derived(root?column.name:memberLabel(shape).replace(/^\./,""));
</script>
{#if ambiguous.length>1}
  <div class="module ambiguous"><strong>{title}</strong><small>同名 {ambiguous.length} 项，无法唯一对应</small><details><summary>已记录数据</summary><pre>{JSON.stringify(ambiguous,null,2)}</pre></details></div>
{:else if compound}
  <section class="module compound" class:chosen class:root class:absent={!value&&root}>
    <button class="heading" aria-pressed={chosen} onclick={()=>onSelect(column)} title={"关注 "+columnLabel(column)}>
      <span>{title}{shape.kind==="array"&&shape.arrayLength!==undefined?"["+shape.arrayLength+"]":""}</span>
      <small>{value?value.status==="observed"?value.children_complete===false?"部分 · "+label(value):value.type:displayValue(value):root?"—":"?"}</small>
      {#if diff}<span class="dot" title="与前一观察有已知差异"></span>{/if}
    </button>
    {#if tabular}
      <div class="inner-scroll"><table><thead><tr><th>下标</th>{#each fields as field}<th>{memberLabel(field).replace(/^\./,"")}</th>{/each}</tr></thead>
        <tbody>{#each shape.children as row}
          {@const rv=childValue(value,row)}{@const rp=childValue(previous,row)}{@const rc=member(column,row)}
          <tr><th><button class="index" class:chosen={active===columnKey(rc)} aria-pressed={active===columnKey(rc)} onclick={()=>onSelect(rc)}>{row.index??row.name}</button></th>
            {#each fields as field}
              {@const v=childValue(rv,field)}{@const p=childValue(rp,field)}{@const c=member(rc,field)}
              <td><button class="cell" class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} class:unknown={!v||v.status!=="observed"} aria-pressed={active===columnKey(c)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+displayValue(v)+(observedChange(p,v)?"（之前 "+displayValue(p)+"）":"")}>{displayValue(v)}</button></td>
            {/each}
          </tr>
        {/each}</tbody>
      </table></div>
    {:else if shape.kind==="array"&&shape.children.every(c=>!c.children.length&&!["array","struct","union"].includes(c.kind))}
      <div class="array inner-scroll">
        {#each shape.children as ch}
          {@const c=member(column,ch)}{@const v=childValue(value,ch)}{@const p=childValue(previous,ch)}
          <button class="element" class:chosen={active===columnKey(c)} class:diff={observedChange(p,v)} class:unknown={!v||v.status!=="observed"} aria-pressed={active===columnKey(c)} onclick={()=>onSelect(c)} title={columnLabel(c)+": "+displayValue(v)+(observedChange(p,v)?"（之前 "+displayValue(p)+"）":"")}>
            <small>{ch.index??ch.name}</small><span>{displayValue(v)}</span>
          </button>
        {/each}
      </div>
    {:else}
      <div class="members inner-scroll">
        {#each shape.children as ch}
          <Self shape={ch} column={member(column,ch)} value={childValue(value,ch)} previous={childValue(previous,ch)} {active} {onSelect}/>
        {/each}
        {#if !shape.children.length}<small>没有已记录成员</small>{/if}
      </div>
    {/if}
  </section>
{:else}
  <button class="module scalar" class:chosen class:diff class:root class:unknown={!value||value.status!=="observed"} aria-pressed={chosen} onclick={()=>onSelect(column)} title={columnLabel(column)+" · "+shape.type+": "+displayValue(value,root)+(diff?"（之前 "+displayValue(previous)+"）":"")}>
    <span class="scalar-name">{title}</span><span class="number">{displayValue(value,root)}</span>
    <small class="before">{diff?displayValue(previous)+" →":value?.value_truncated?"截断":""}</small>
  </button>
{/if}
<style>
  .module{border:1px solid var(--line);border-radius:5px;background:var(--panel);min-width:0;color:var(--text)}
  .chosen,.module.chosen{outline:2px solid var(--accent);outline-offset:-2px}
  button{font:inherit;color:inherit;cursor:pointer;border:0;background:transparent}
  button:hover{background:var(--hover)}button:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}
  .scalar{display:grid;grid-template-columns:minmax(0,1fr) auto;grid-template-rows:20px 24px 15px;align-items:center;text-align:left;padding:5px 10px;width:100%;min-width:0}
  .scalar-name{grid-column:1/-1;font-size:12px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .number{grid-column:1/-1;font:600 19px/1.2 var(--mono);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .before{grid-column:1/-1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .diff{color:var(--accent);background:var(--soft)}
  .unknown .number,.unknown.cell,.unknown.element{color:var(--muted)}
  small{color:var(--muted);font-size:11px}
  .heading{display:flex;align-items:center;gap:8px;width:100%;text-align:left;padding:8px 10px;border-bottom:1px solid var(--line);font:600 13px var(--mono);min-height:35px}
  .heading small{font:11px system-ui;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-left:auto}
  .dot{background:var(--accent);height:6px;width:6px;border-radius:100%;flex:none}
  .inner-scroll{max-height:290px;overflow:auto;min-width:0}
  .array{display:grid;grid-template-columns:repeat(auto-fill,minmax(70px,1fr));padding:5px;gap:3px}
  .element{display:flex;flex-direction:column;align-items:center;justify-content:center;border:1px solid var(--line);border-radius:3px;min-height:45px;padding:4px;min-width:0;font:14px var(--mono)}
  .element span{max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  table{border-collapse:separate;border-spacing:0;width:100%;font:13px var(--mono)}
  th{font-weight:400;color:var(--muted);background:var(--panel);white-space:nowrap;font-size:11px}
  thead th{position:sticky;top:0;z-index:1;padding:5px 8px;border-bottom:1px solid var(--line)}
  td,tbody th{border-bottom:1px solid var(--line);border-right:1px solid var(--line);padding:0;text-align:center}
  tbody tr:last-child td,tbody tr:last-child th{border-bottom:0}
  .cell,.index{display:block;width:100%;min-width:58px;min-height:29px;padding:4px 8px;font-family:var(--mono);white-space:nowrap}
  .index{min-width:32px;color:var(--muted)}
  .members{display:grid;grid-template-columns:repeat(auto-fit,minmax(110px,1fr));gap:5px;padding:5px;align-items:start}
  .members :global(.compound){grid-column:1/-1}
  .ambiguous{padding:8px}.ambiguous small{display:block}.ambiguous pre{max-height:200px;overflow:auto;white-space:pre-wrap}
</style>
