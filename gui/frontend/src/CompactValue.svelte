<script lang="ts">
  import {atPath, label, valueChanged, differences, previewValues, type Variable, type LeafSlot} from "./observation";
  let {value,previous,slots=[],onInspect}: {value:Variable;previous?:Variable;slots?:LeafSlot[];onInspect:()=>void}=$props();
  let diffs=$derived(differences(previous,value,"",3));
  let preview=$derived(previewValues(value,3));
</script>
{#if slots.length}
  <button class="slots" style:grid-template-columns={"repeat("+slots.length+",minmax(40px,1fr))"} onclick={onInspect} title={value.type+" · "+label(value)}>
    {#each slots as slot}
      {@const v=atPath(value,slot.path)}
      {@const p=atPath(previous,slot.path)}
      <span class:diff={!!v&&valueChanged(p,v)} class:muted={!v||v.status!=="observed"} title={slot.label+": "+(v?label(v):"未记录")+(p&&v&&valueChanged(p,v)?"（之前 "+label(p)+"）":"")}>{v?v.status==="not_captured"?"?":v.status==="unavailable"?"!":label(v):"?"}</span>
    {/each}
  </button>
{:else if value.status==="observed"&&value.children?.length}
  <button class="compound" onclick={onInspect} title={value.type+" · "+label(value)}>
    {#if diffs.length}
      {#each diffs.slice(0,2) as d}<span class="diff">{d.path}: {d.before} → {d.after}</span>{/each}
      {#if diffs.length>2}…{/if}
    {:else}
      {#each preview.slice(0,2) as p}<span>{p.path}: {p.value}</span>{/each}
      {#if preview.length>2}…{/if}
    {/if}
    {#if value.children_complete===false}<small>部分</small>{/if}
  </button>
{:else}
  <button class="scalar" class:diff={valueChanged(previous,value)} class:muted={value.status!=="observed"} onclick={onInspect} title={value.type}>{#if valueChanged(previous,value)}<small>{label(previous!)} → </small>{/if}{label(value)}</button>
{/if}
<style>
  button{border:0;background:transparent;color:inherit;font:inherit;padding:0;text-align:left;cursor:pointer;max-width:100%}
  button:hover{outline:1px solid var(--line);border-radius:3px}
  .slots{display:grid;gap:0;width:100%;font-variant-numeric:tabular-nums}
  .slots span{padding:0 4px;text-align:center;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;border-right:1px solid var(--line)}
  .slots span:last-child{border:0}
  .compound{display:flex;align-items:center;gap:6px;white-space:nowrap;max-width:290px;overflow:hidden;text-overflow:ellipsis}
  .scalar{white-space:nowrap;max-width:180px;overflow:hidden;text-overflow:ellipsis}
  .muted,small{color:var(--muted)}
  .diff{color:var(--accent);font-weight:600;background:var(--soft);border-radius:3px}
</style>
