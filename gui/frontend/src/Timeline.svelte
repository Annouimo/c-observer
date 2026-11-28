<script lang="ts">
  import {tick} from "svelte";
  import {trackPosition,markerHit,navigationTarget,markerPath,type NavigationMode} from "./observation";
  let {step,total,events,focus,onGo}:{step:number;total:number;events:number[];focus:string;onGo:(id:number,mode?:NavigationMode)=>void}=$props();
  let marks=$derived(markerPath(events,total));
  let progress=$derived(total>1?step/(total-1)*100:0);
  let track:HTMLDivElement;
  let edit:HTMLInputElement;
  let editing=$state(false);
  let draft=$state("");
  let pointer:number|null=null,startX=0,startY=0,moved=false,wheelSum=0,wheelAt=0;
  function position(e:PointerEvent){
    const r=track.getBoundingClientRect();
    return {p:trackPosition(e.clientX-r.left-8,r.width-16,total),width:r.width-16,band:e.clientY-r.top<19};
  }
  function down(e:PointerEvent){
    if(total<2||e.button!==0||pointer!==null)return;
    e.preventDefault();track.focus({preventScroll:true});pointer=e.pointerId;startX=e.clientX;startY=e.clientY;moved=false;track.setPointerCapture(e.pointerId);
  }
  function move(e:PointerEvent){
    if(pointer!==e.pointerId)return;
    if(Math.hypot(e.clientX-startX,e.clientY-startY)>3)moved=true;
    if(moved)onGo(Math.round(position(e).p),"drag");
  }
  function up(e:PointerEvent){
    if(pointer!==e.pointerId)return;
    const {p,width,band}=position(e);
    pointer=null;
    if(track.hasPointerCapture(e.pointerId))track.releasePointerCapture(e.pointerId);
    onGo(!moved&&band?(markerHit(events,p,total,width)??Math.round(p)):Math.round(p),moved?"drag":"direct");
  }
  function cancel(){pointer=null;}
  function keys(e:KeyboardEvent){
    if(e.ctrlKey||e.metaKey||e.altKey)return;
    const n=navigationTarget(e.key,e.shiftKey,step,total,events);
    if(n!==undefined){e.preventDefault();e.stopPropagation();onGo(n);}
  }
  function wheel(node:HTMLElement){
    function handle(e:WheelEvent){
      if(total<2||e.ctrlKey||e.metaKey)return;
      e.preventDefault();
      const now=performance.now();
      if(now-wheelAt>180)wheelSum=0;
      wheelAt=now;
      const raw=Math.abs(e.deltaX)>Math.abs(e.deltaY)?e.deltaX:e.deltaY;
      const amount=raw*(e.deltaMode===1?16:e.deltaMode===2?120:1);
      if(Math.sign(amount)!==Math.sign(wheelSum))wheelSum=0;
      wheelSum+=amount;
      if(Math.abs(wheelSum)>=22){const d=Math.sign(wheelSum);wheelSum=0;onGo(step+d,"drag");}
    }
    node.addEventListener("wheel",handle,{passive:false});
    return {destroy(){node.removeEventListener("wheel",handle);}};
  }
  async function begin(){
    if(!total)return;
    draft=String(step+1);editing=true;await tick();edit?.focus();edit?.select();
  }
  function finish(commit:boolean){
    if(!editing)return;
    if(commit&&/^\d+$/.test(draft.trim()))onGo(Number(draft)-1,"seek");
    editing=false;
  }
</script>
<footer>
  <div class="head">
    <span class="focus" title={focus?events.length+" 次已知变化":""}>{focus}{#if focus&&!events.length}<small> · 无已知变化</small>{/if}</span>
    {#if editing}
      <label class="position"><input bind:this={edit} bind:value={draft} inputmode="numeric" aria-label="观察编号" onblur={()=>finish(true)} onkeydown={e=>{if(e.key==="Enter"){e.preventDefault();finish(true);track.focus();}else if(e.key==="Escape"){e.preventDefault();finish(false);track.focus();}}}/> / {total}</label>
    {:else}
      <button class="position" disabled={!total} onclick={begin} title="输入观察编号">{total?step+1:0} / {total}</button>
    {/if}
  </div>
  <div class="track" bind:this={track} role="slider" tabindex={total?0:-1} aria-label="观察时间线" aria-valuemin={total?1:0} aria-valuemax={total} aria-valuenow={total?step+1:0} aria-disabled={total<2} title="拖动定位 · 点击上方标记跳转 · 滚轮微调 · Shift + 方向键切换变化"
    onpointerdown={down} onpointermove={move} onpointerup={up} onpointercancel={cancel} onlostpointercapture={cancel} onkeydown={keys} use:wheel>
    <div class="rail"><div class="progress" style:width={progress+"%"}></div></div>
    <svg class="markers" viewBox="0 0 1000 12" preserveAspectRatio="none" aria-hidden="true">{#key focus+"\n"+marks}<path class="mark-reveal" d={marks} vector-effect="non-scaling-stroke"/>{/key}</svg>
    <div class="thumb-zone"><div class="thumb" style:left={progress+"%"}></div></div>
  </div>
</footer>
<style>
  footer{flex:none;background:var(--panel);border-top:1px solid var(--line);padding:7px 14px 8px}
  .head{display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:24px}
  .focus{color:var(--accent);font:600 12px var(--mono);min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  small{font:11px system-ui;color:var(--muted);font-weight:400}
  .position{font:12px var(--mono);font-variant-numeric:tabular-nums;color:var(--muted);border:0;background:transparent;padding:3px 5px;white-space:nowrap;cursor:pointer}
  .position:hover:not(:disabled){background:var(--hover);border-radius:var(--ui-radius-sm)}
  .position input{width:65px;font:inherit;color:var(--text);background:var(--panel);border:1px solid var(--line);border-radius:4px;text-align:right}
  .track{position:relative;height:36px;cursor:ew-resize;touch-action:none;user-select:none}
  .track[aria-disabled="true"]{cursor:default}
  .rail{position:absolute;left:8px;right:8px;top:21px;height:4px;border-radius:3px;background:var(--line);pointer-events:none}
  .progress{height:100%;background:var(--accent);opacity:.32;border-radius:inherit}
  .markers{position:absolute;left:8px;width:calc(100% - 16px);height:12px;top:8px;overflow:visible;pointer-events:none}
  .markers path{stroke:var(--accent);stroke-width:1.6;fill:none}
  .mark-reveal{animation:mark-reveal 180ms ease-out}
  @keyframes mark-reveal{from{opacity:0}to{opacity:1}}
  @media(prefers-reduced-motion:reduce){.mark-reveal{animation:none}}
  .thumb-zone{position:absolute;left:8px;right:8px;top:23px;pointer-events:none}
  .thumb{position:absolute;width:16px;height:16px;border-radius:50%;background:var(--accent);border:2px solid var(--panel);box-shadow:0 0 0 1px var(--accent);transform:translate(-50%,-50%)}
  .track:focus-visible{outline:none}
  .track:focus-visible .thumb{box-shadow:0 0 0 1px var(--accent),0 0 0 4px var(--panel),0 0 0 5.5px var(--accent)}
  .position:focus-visible,.position input:focus-visible{outline:var(--ui-focus-width) solid var(--ui-focus);outline-offset:-1px;border-radius:4px}
  @media(forced-colors:active){.track:focus-visible .thumb{outline:2px solid Highlight;outline-offset:3px}}
</style>
