<script lang="ts">
 let count = $state(0);
 let name = $state('');
 let enabled = $state(true);
 let value = $state(40);
 let dark = $state(false);
 let tab = $state('controls');
 const rows = Array.from({length: 30}, (_, i) => ({ index: i, name: '测试数据 ' + (i+1), value: (i+1)*10 }));
 const code = '#include <stdio.h>\n\nint main(void) {\n    printf("Hello, World!\\n");\n    return 0;\n}';
</script>

<div class:dark class="page">
 <main>
  <header>
   <div><small>TECHNICAL EXPERIMENT · 01</small><h1>WebView Playground</h1><p>Wails v3 · Svelte · TypeScript · 本地资源</p></div>
   <button onclick={() => dark = !dark}>{dark ? '浅色模式' : '深色模式'}</button>
  </header>
  <nav aria-label="实验页面">
   <button class:active={tab==='controls'} onclick={() => tab='controls'}>基础组件</button>
   <button class:active={tab==='table'} onclick={() => tab='table'}>表格与滚动</button>
   <button class:active={tab==='code'} onclick={() => tab='code'}>等宽文本</button>
  </nav>
  {#if tab==='controls'}
   <section>
    <h2>基础交互</h2>
    <div class="row"><button class="primary" onclick={() => count++}>点击按钮</button><button onclick={() => count=0}>重置</button><output>Count: {count}</output></div>
    <label>文本输入（测试中文输入法）<input placeholder="输入任意文字" bind:value={name}></label>
    <p>输入内容：{name || '—'}</p>
    <label class="row"><input type="checkbox" bind:checked={enabled}>启用选项 <output>{enabled ? '已启用' : '已关闭'}</output></label>
    <label>滑块 · {value}%<input type="range" min="0" max="100" bind:value={value}></label>
    <progress max="100" value={value}></progress>
   </section>
   <div class="grid">
    {#each [['HTML','布局'],['CSS','样式'],['JavaScript','交互']] as item}
     <section><h2>{item[0]}</h2><p>{item[1]}</p></section>
    {/each}
   </div>
  {:else if tab==='table'}
   <section><h2>表格与独立滚动容器</h2><div class="scroll"><table><thead><tr><th>Index</th><th>Name</th><th>Value</th></tr></thead><tbody>{#each rows as row}<tr><td>{row.index}</td><td>{row.name}</td><td>{row.value}</td></tr>{/each}</tbody></table></div></section>
  {:else}
   <section><h2>等宽文本与空白保留</h2><pre>{code}</pre></section>
  {/if}
  <footer>仅验证原生窗口与 Web 组件 · 不包含 Observer 业务逻辑</footer>
 </main>
</div>

<style>
 :global(*){box-sizing:border-box}
 :global(body){margin:0;font-family:system-ui,-apple-system,"Segoe UI",sans-serif}
 .page{--bg:#f5f6f8;--card:#fff;--text:#18202c;--muted:#626c7a;--line:#d9dfe7;--accent:#245cba;min-height:100vh;background:var(--bg);color:var(--text);padding:32px;color-scheme:light}
 .page.dark{--bg:#15191f;--card:#222831;--text:#f0f3f7;--muted:#aab3c0;--line:#3c4654;--accent:#8ab5ff;color-scheme:dark}
 main{max-width:940px;margin:auto}
 header,.row{display:flex;align-items:center;gap:12px;flex-wrap:wrap}
 header{justify-content:space-between}
 h1{margin:8px 0;font-size:clamp(24px,4vw,36px)}
 h2{font-size:18px;margin:0 0 16px}
 p,small,footer{color:var(--muted)}
 small{letter-spacing:.12em}
 nav{display:flex;flex-wrap:wrap;gap:8px;margin:20px 0}
 button,input:not([type=checkbox]):not([type=range]){font:inherit;color:inherit;background:var(--card);border:1px solid var(--line);border-radius:7px;padding:9px 14px}
 button{cursor:pointer}
 button.active,button.primary{border-color:var(--accent);color:var(--accent);font-weight:600}
 section{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:20px;margin-bottom:12px}
 label:not(.row){display:grid;gap:8px;margin-top:16px}
 input[type=range],progress{width:100%;accent-color:var(--accent)}
 output{font-variant-numeric:tabular-nums;margin-left:auto}
 .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px}
 .grid p{margin:0}
 .scroll{max-height:340px;overflow:auto;border:1px solid var(--line);border-radius:6px}
 table{border-collapse:collapse;width:100%;text-align:left}
 th,td{padding:10px 14px;border-bottom:1px solid var(--line)}
 th{position:sticky;top:0;background:var(--card)}
 pre{overflow:auto;font:15px/1.7 ui-monospace,SFMono-Regular,Consolas,monospace;padding:16px;background:var(--bg);border-radius:6px}
 footer{font-size:12px;margin-top:16px}
 @media(max-width:600px){.page{padding:16px}section{padding:14px}}
</style>
