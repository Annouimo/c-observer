import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseDocument, tokenize, groupTrace, flatten, values, changed, label, valueChanged, sourceDocument, differences, memberColumns, groupDifferences, loopRanges, replacedLines, smallLayout, atPath, projectRounds, slotSummary, slotHeaders, type Run } from "./observation.ts";
const fixture=JSON.parse(readFileSync(new URL("../../../testdata/snapshots/wrong-index.json",import.meta.url),"utf8"));
test("fixture is explicitly synthetic and preserves observed values",()=>{
  const doc=parseDocument(fixture);
  assert.equal(doc.synthetic,true);
  assert.equal(values(doc.run.snapshots[1],{name:"index",fn:"main"})[0].value,"2");
  assert.equal(values(doc.run.snapshots[1],{name:"index",fn:"other"}).length,0);
  assert.match(label(values(doc.run.snapshots[1],{name:"a",fn:"main"})[0]),/3\/4/);
  assert.equal(changed(doc.run,doc.run.snapshots[1],{name:"index",fn:"main"}),false);
  assert.equal(changed(doc.run,doc.run.snapshots[3],{name:"i",fn:"main"}),true);
});
test("lexical hints ignore comments and literals",()=>{
  const tokens=tokenize('/* for\nwhile */\nchar *s="while";\nfor (;;) {}');
  assert.equal(tokens.length,4);
  assert.deepEqual(tokens.flat().filter(t=>t.kind==="keyword"&&["for","while"].includes(t.text)).map(t=>t.text),["for"]);
});
test("grouping preserves all original observations, including partial fixture",()=>{
  const {run}=parseDocument(fixture);
  assert.deepEqual(flatten(groupTrace(run)),run.snapshots);
});
test("repeat grouping retains last anchor and never invents states",()=>{
  const run:Run={...parseDocument(fixture).run,session:{backend:"synthetic",sources:[{path:"x.c",text:"while (i) {\n i--;\n}\n"}]},snapshots:[1,2,1,2,1,3].map((line,id)=>({id,location:{file:"x.c",line,function:"main"},frames:[] }))};
  const tree=groupTrace(run);
  assert.equal(tree[0].kind,"group");
  assert.deepEqual(flatten(tree),run.snapshots);
});
test("reject invalid record and duplicate ids",()=>{
  assert.throws(()=>parseDocument({}));
  const invalid=structuredClone(fixture);invalid.run.snapshots[0].id=9;
  assert.throws(()=>parseDocument(invalid));
});
test("v1 byte input differs from fixture plain text",()=>{
  const x=structuredClone(fixture);delete x.synthetic;x.format="c-observer-record";x.schema_version=1;x.run.session.backend="gdb-mi";x.stdin=btoa("abc\n");x.expected_stdout=null;
  assert.equal(parseDocument(x).stdin,"abc\n");assert.equal(parseDocument(x).expected,null);
});

test("partial array differences are evidence only at matching observed indices",()=>{
  const {run}=parseDocument(fixture);
  const a=values(run.snapshots[1],{name:"a",fn:"main"})[0];
  const b=structuredClone(a);
  b.children![0].children![0].value="999";
  assert.equal(valueChanged(a,b),true);
  b.children![0].children![0].status="not_captured";
  assert.equal(valueChanged(a,b),false);
});
test("Go nil slices in a failed run are accepted",()=>{
  const x=structuredClone(fixture);delete x.synthetic;x.format="c-observer-record";x.schema_version=1;x.run.session.backend="gdb-mi";
  x.stdin="";x.expected_stdout=null;x.run.snapshots=null;x.run.session.sources=null;x.run.reason="compile_failed";
  assert.equal(parseDocument(x).run.snapshots.length,0);
});

test("static skeleton retains all lines of sparse wrong-index fixture",()=>{
  const {run}=parseDocument(fixture);
  const tree=groupTrace(run);
  const loop=tree.find(n=>n.kind==="group"&&n.groupType==="loop");
  assert.ok(loop&&loop.kind==="group");
  assert.equal(loop.anchor.line,7);
  assert.equal(loop.children.length,2);
  assert.deepEqual(flatten(loop.children).map(s=>s.location.line),[8,9,7,8,9,7]);
  assert.deepEqual(flatten(tree),run.snapshots);
  const {rows}=sourceDocument(run,"main.c",tree);
  assert.equal(rows.length,run.session.sources[0].text.split(/\r?\n/).length);
  assert.equal(rows[0].tokens.map(t=>t.text).join(""),"#include <stdio.h>");
  assert.equal(rows[4].nodes.length,0);
  assert.equal(rows[6].nodes[0],loop);
  assert.equal(rows[7].count,2);
  assert.equal(rows[7].nodes.length,0);
  assert.equal(groupDifferences(run,loop,{name:"a",fn:"main"})[0].path,"[0].score");
});
test("member columns retain recorded indices and differences",()=>{
  const {run}=parseDocument(fixture), c={name:"a",fn:"main"};
  const v=values(run.snapshots[1],c)[0], child=memberColumns(v,c)[0];
  const member=memberColumns(v.children![0],child)[0];
  assert.equal(values(run.snapshots[2],member)[0].value,"17");
  assert.deepEqual(differences(v,values(run.snapshots[2],c)[0]),[{path:"[0].score",before:"10",after:"17"}]);
});
test("nested calls preserve raw order and anchor to caller",()=>{
  const run:Run={...parseDocument(fixture).run,session:{backend:"synthetic",sources:[{path:"x.c",text:"int f(){\n return 1;\n}\nint main(){\n f();\n return 0;\n}\n"}]},snapshots:[{line:5,fn:"main",depth:1},{line:2,fn:"f",depth:2},{line:6,fn:"main",depth:1}].map((x,id)=>({id,location:{file:"x.c",line:x.line,function:x.fn},frames:Array.from({length:x.depth},()=>({location:{file:"x.c",line:x.line,function:x.fn},variables:[]}))}))};
  const tree=groupTrace(run);
  assert.deepEqual(flatten(tree),run.snapshots);
  const row=sourceDocument(run,"x.c",tree).rows[4];
  assert.equal(row.nodes.length,2);assert.equal(row.nodes[1].kind,"group");
});
test("loop ranges ignore braces in literals and comments",()=>{
  assert.deepEqual(loopRanges({path:"x.c",text:'// for (;;) {\nfor (;;) {\n puts("}");\n}\n'}),[{file:"x.c",start:2,end:4}]);
});

test("replacement hides represented loop body only while open",()=>{
  const {run}=parseDocument(fixture), {rows}=sourceDocument(run,"main.c");
  assert.deepEqual([...replacedLines(rows,()=>true)],[8,9]);
  assert.equal(replacedLines(rows,()=>false).size,0);
  assert.ok(rows.some(r=>r.line===10));
});
test("stable small layout includes missing index and never invents value",()=>{
  const {run}=parseDocument(fixture), c={name:"a",fn:"main"};
  const all=run.snapshots.flatMap(s=>values(s,c)), slots=smallLayout(all);
  assert.deepEqual(slots.map(s=>s.label),["[0].score","[1].score","[2].score","[3].score"]);
  assert.equal(atPath(all[0],slots[3].path),undefined);
  assert.equal(atPath(all[2],slots[0].path)?.value,"17");
  assert.deepEqual(smallLayout([{...all[0],array_length:10000}]),[]);
});
test("group summary excludes changes entering the group",()=>{
  const {run}=parseDocument(fixture);
  const loop=groupTrace(run).find(n=>n.kind==="group"&&n.groupType==="loop");
  assert.ok(loop&&loop.kind==="group");
  assert.deepEqual(groupDifferences(run,loop.children[1],{name:"i",fn:"main"}),[]);
  assert.equal(groupDifferences(run,loop.children[1],{name:"a",fn:"main"})[0].path,"[1].score");
});

test("round selection keeps source rows stable and does not copy values from other groups",()=>{
  const {run}=parseDocument(fixture),tree=groupTrace(run);
  const first=projectRounds(tree,{},[]);
  const loop=first.loops[0];
  assert.ok(loop);
  assert.deepEqual(flatten(first.nodes).map(s=>s.id),[0,1,2,6]);
  const second=projectRounds(tree,{[loop.key]:1},[]);
  assert.deepEqual(flatten(second.nodes).map(s=>s.id),[3,4,5,6]);
  const a=sourceDocument(run,"main.c",first.nodes),b=sourceDocument(run,"main.c",second.nodes);
  assert.deepEqual(a.rows.map(r=>r.line),b.rows.map(r=>r.line));
  assert.equal(a.rows[7].nodes[0].kind,"observation");
  assert.equal(b.rows[7].nodes[0].kind,"observation");
  assert.deepEqual(flatten(projectRounds(tree,{},[loop.key]).nodes),run.snapshots);
});
test("nested loop selectors belong only to the selected outer group",()=>{
  const {run}=parseDocument(fixture);
  const obs=(id:number)=>({kind:"observation" as const,snapshot:run.snapshots[id]});
  const anchor=run.snapshots[0].location;
  const group=(key:string,groupType:"loop"|"round",children:any[])=>({kind:"group" as const,key,groupType,anchor,label:key,children});
  const inner=group("inner","loop",[group("i0","round",[obs(0)]),group("i1","round",[obs(1)])]);
  const outer=group("outer","loop",[group("o0","round",[inner]),group("o1","round",[obs(2)])]);
  assert.deepEqual(flatten(projectRounds([outer],{inner:1},[]).nodes).map(s=>s.id),[1]);
  assert.deepEqual(projectRounds([outer],{outer:1},[]).loops.map(n=>n.key),["outer"]);
});
test("fixed array summary follows slot coordinates, including missing indices",()=>{
  const {run}=parseDocument(fixture),c={name:"a",fn:"main"};
  const loop=projectRounds(groupTrace(run),{},[]).loops[0];
  const slots=smallLayout(run.snapshots.flatMap(s=>values(s,c)));
  assert.deepEqual(slotHeaders(slots),{suffix:"[].score",labels:["[0]","[1]","[2]","[3]"]});
  assert.deepEqual(slots.map(slot=>slotSummary(run,loop,c,slot).text),["10 → 17","20 → 25","30","?"]);
});
