<script lang="ts">
  import { label, childBefore, valueChanged, type Variable } from "./observation";
  import Self from "./DataValue.svelte";
  let { value, previous }: {value: Variable; previous?: Variable} = $props();
  let records = $derived(value.kind === "array" && !!value.children?.length && value.children.every(c => c.kind === "struct" && c.children?.length));
  let fields = $derived(records ? [...new Set(value.children!.flatMap(c=>c.children!.map(m=>m.name)))] : []);
</script>
{#if value.children?.length}
  <details>
    <summary title={value.type}>{label(value)}</summary>
    {#if records}
      <div class="nested-scroll"><table class="nested"><thead><tr><th>下标</th>{#each fields as field}<th>{field}</th>{/each}</tr></thead>
        <tbody>{#each value.children as child}<tr><th>{child.index ?? child.name}</th>{#each fields as field}<td>{#each child.children?.filter(m=>m.name===field)??[] as member}<Self value={member} previous={childBefore(childBefore(previous,child),member)}/>{:else}<span class="muted">未记录</span>{/each}</td>{/each}</tr>{/each}</tbody></table></div>
    {:else}
      <div class="members">{#each value.children as child}<div><span class="muted">{child.index === undefined ? child.name : '['+child.index+']'}</span><Self value={child} previous={childBefore(previous,child)}/></div>{/each}</div>
    {/if}
    {#if value.children_complete === false}<small>仅展示已记录成员</small>{/if}
  </details>
{:else}
  <span class:muted={value.status!=="observed"} class:diff={valueChanged(previous,value)} title={valueChanged(previous,value)?label(previous!)+" → "+label(value):value.type}>{label(value)}{#if valueChanged(previous,value)} <small>← {label(previous!)}</small>{/if}</span>
{/if}
<style>
  summary{cursor:pointer;color:var(--accent);white-space:nowrap}
  .members{display:grid;gap:4px;margin-top:6px;padding-left:8px;border-left:1px solid var(--line)}
  .members>div{display:flex;gap:10px;align-items:baseline}
  .nested-scroll{overflow:auto;max-width:450px;max-height:280px}
  .nested{border-collapse:collapse;font-size:12px;width:100%}
  .nested th,.nested td{border:1px solid var(--line);padding:4px;text-align:left;vertical-align:top}
  .muted,small{color:var(--muted)} .diff{background:var(--change);border-radius:3px;font-weight:600;padding:0 3px}
</style>
