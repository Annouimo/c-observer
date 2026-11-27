<script lang="ts">
  import { label, childBefore, valueChanged, differences, previewValues, memberColumns, columnLabel, type Variable, type Column } from "./observation";
  import Self from "./DataValue.svelte";
  let { value, previous, column, onSelect }: {value:Variable; previous?:Variable; column?:Column; onSelect?:(c:Column)=>void}=$props();
  let diffs=$derived(differences(previous,value,"",4));
  let preview=$derived(previewValues(value,5));
  let members=$derived(column?memberColumns(value,column):[]);
  let records=$derived(value.kind==="array"&&!!value.children?.length&&value.children.every(c=>c.kind==="struct"&&c.children?.length));
  let fields=$derived(records?[...new Set(value.children!.flatMap(c=>c.children!.map(m=>m.name)))]:[]);
</script>
{#if value.status==="observed"&&value.children?.length}
  <details>
    <summary title={value.type+" · "+label(value)}>
      {#if diffs.length}
        {#each diffs.slice(0,2) as d}<span class="diff">{d.path}: {d.before} → {d.after}</span>{/each}
        {#if diffs.length>2}<small>…</small>{/if}
      {:else}
        {#each preview.slice(0,4) as p}<span class="preview">{p.path}: {p.value}</span>{/each}
        {#if preview.length>4}<small>…</small>{/if}
      {/if}
      {#if value.children_complete===false}<small class="partial" title={label(value)}>部分</small>{/if}
    </summary>
    {#if records}
      <div class="nested-scroll"><table class="nested"><thead><tr><th>下标</th>{#each fields as field}<th>{field}</th>{/each}</tr></thead>
        <tbody>{#each value.children as child,i}<tr><th>{#if members[i]&&onSelect}<button onclick={()=>onSelect?.(members[i])} title="添加元素观察列">{child.index??child.name} +</button>{:else}{child.index??child.name}{/if}</th>{#each fields as field}<td>{#each child.children?.filter(m=>m.name===field)??[] as member}
          {#if members[i]&&onSelect}<button class="add" title="添加成员观察列" onclick={()=>onSelect?.({...members[i],path:[...(members[i].path??[]),{name:member.name,index:member.index}]})}>+</button>{/if}
          <Self value={member} previous={childBefore(childBefore(previous,child),member)} />
        {:else}<span class="muted">未记录</span>{/each}</td>{/each}</tr>{/each}</tbody></table></div>
    {:else}
      <div class="members">{#each value.children as child,i}<div>
        {#if members[i]&&onSelect}<button onclick={()=>onSelect?.(members[i])} title={"添加 "+columnLabel(members[i])}>{child.index===undefined?child.name:"["+child.index+"]"} +</button>{:else}<span class="muted">{child.index===undefined?child.name:"["+child.index+"]"}</span>{/if}
        <Self value={child} previous={childBefore(previous,child)} column={members[i]} {onSelect}/>
      </div>{/each}</div>
    {/if}
    {#if value.children_complete===false}<small>{label(value)}，其余未记录</small>{/if}
  </details>
{:else}
  <span class:muted={value.status!=="observed"} class:diff={valueChanged(previous,value)} title={value.type}>{#if valueChanged(previous,value)}<small>{label(previous!)} → </small>{/if}{label(value)}</span>
{/if}
<style>
  details{max-width:390px}
  summary{cursor:pointer;white-space:normal;line-height:1.5}
  .preview,.diff{display:inline-block;margin-right:6px;white-space:nowrap;font-variant-numeric:tabular-nums}
  .members{display:grid;gap:3px;margin-top:5px;padding-left:8px;border-left:1px solid var(--line)}
  .members>div{display:flex;gap:8px;align-items:baseline}
  .nested-scroll{overflow:auto;max-width:390px;max-height:260px}
  .nested{border-collapse:collapse;font-size:12px;width:100%}
  .nested th,.nested td{border:1px solid var(--line);padding:3px;text-align:left;vertical-align:top}
  button{border:0;background:none;color:var(--accent);cursor:pointer;font:inherit;padding:0 3px}
  button:hover{text-decoration:underline}
  .muted,small{color:var(--muted)} .diff{color:var(--accent);background:var(--soft);border-radius:3px;font-weight:600;padding:0 3px}
  .partial{white-space:nowrap;font-size:11px}
</style>
