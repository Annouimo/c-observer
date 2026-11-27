import {test} from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {parseDocument,modules,member,selectedValues,matchingPrevious,observedChange,changeEvents,nextEvent,displayValue,sharedFields,columnKey,outputText} from "./observation.ts";
const fixture=JSON.parse(readFileSync(new URL("../../../testdata/snapshots/wrong-index.json",import.meta.url),"utf8"));
const run=()=>parseDocument(structuredClone(fixture)).run;
test("raw observation ids and actual data retained",()=>{
  const r=run();assert.deepEqual(r.snapshots.map(s=>s.id),[0,1,2,3,4,5,6]);
  assert.equal(selectedValues(r.snapshots[1],{name:"index",fn:"main"})[0].value,"2");
  assert.equal(selectedValues(r.snapshots[6],{name:"i",fn:"main"}).length,0);
  assert.equal(outputText(r.output.stdout),"17 25 30 40\n");
});
test("fixed coordinates include missing declared array slot without inventing data",()=>{
  const r=run(),m=modules(r).find(m=>m.column.name==="a")!;
  assert.deepEqual(m.shape.children.map(c=>c.index),[0,1,2,3]);
  assert.deepEqual(sharedFields(m.shape).map(c=>c.name),["score"]);
  const c=member(member(m.column,m.shape.children[3]),sharedFields(m.shape)[0]);
  assert.equal(selectedValues(r.snapshots[6],c).length,0);
  assert.equal(displayValue(undefined),"?");assert.equal(displayValue(undefined,true),"—");
});
test("root and member events are actual adjacent differences, never first acquisition",()=>{
  const r=run(),a={name:"a",fn:"main"},score=(i:number)=>member(member(a,{name:"ignored",index:i}),{name:"score"});
  assert.deepEqual(changeEvents(r,a),[2,5]);
  assert.deepEqual(changeEvents(r,score(0)),[2]);
  assert.deepEqual(changeEvents(r,score(1)),[5]);
  assert.deepEqual(changeEvents(r,score(3)),[]);
  assert.deepEqual(changeEvents(r,{name:"index",fn:"main"}),[4]);
  assert.deepEqual(changeEvents(r,{name:"i",fn:"main"}),[3]);
  assert.equal(nextEvent([2,5],3,-1),2);assert.equal(nextEvent([2,5],3,1),5);
  assert.equal(nextEvent([2,5],5,1),undefined);
});
test("jumping does not change comparison basis",()=>{
  const r=run(),c={name:"a",fn:"main"},s=r.snapshots[5];
  assert.ok(observedChange(matchingPrevious(r,s,c),selectedValues(s,c)[0]));
  assert.equal(matchingPrevious(r,s,c),selectedValues(r.snapshots[4],c)[0]);
});
test("duplicate names, cross-frame locals and unavailable values do not become changes",()=>{
  const r=run(),c={name:"index",fn:"main"};
  r.snapshots[4].frames[0].variables.push(structuredClone(selectedValues(r.snapshots[4],c)[0]));
  assert.equal(matchingPrevious(r,r.snapshots[4],c),undefined);
  assert.deepEqual(changeEvents(r,c),[]);
  const x=run();x.snapshots[4].frames.push(structuredClone(x.snapshots[4].frames[0]));
  assert.equal(matchingPrevious(x,x.snapshots[4],c),undefined);
  const y=run();selectedValues(y.snapshots[4],c)[0].status="unavailable";
  assert.deepEqual(changeEvents(y,c),[]);
});
test("partial member comparisons do not infer missing values; globals distinct",()=>{
  const r=run(),c={name:"a",fn:"main"};
  const v=selectedValues(r.snapshots[2],c)[0];v.children![0].children![0].status="not_captured";
  assert.equal(changeEvents(r,c).includes(2),false);
  assert.notEqual(columnKey(c),columnKey({name:"a",global:true}));
});
test("reject duplicate ids and accept v1 nil slices",()=>{
  const x=structuredClone(fixture);x.run.snapshots[0].id=9;assert.throws(()=>parseDocument(x));
  const y=structuredClone(fixture);delete y.synthetic;y.format="c-observer-record";y.schema_version=1;y.run.session.backend="gdb-mi";y.stdin="";y.expected_stdout=null;y.run.snapshots=null;y.run.session.sources=null;
  assert.equal(parseDocument(y).run.snapshots.length,0);
});


import {visibleModules,moduleWidth,arraySegments,foldValue,replayContext,markerPath,sampleResult,outputRows,sourceTokens} from "./observation.ts";
const loadFixture=(name:string)=>parseDocument(JSON.parse(readFileSync(new URL("../../../testdata/snapshots/"+name+".json",import.meta.url),"utf8")));
test("replay context uses real interval, never future or inferred source",()=>{
  const r=run();assert.equal(replayContext(r,0).before,undefined);
  assert.equal(replayContext(r,5).origin?.line,9);assert.equal(replayContext(r,5).after?.line,7);
});
test("known frame, object and declaration conflicts block root/member differences",()=>{
  const r=run(),c={name:"a",fn:"main"},m=member(member(c,{name:"[1]",index:1}),{name:"score"});
  selectedValues(r.snapshots[5],c)[0].id="other";
  assert.equal(matchingPrevious(r,r.snapshots[5],m),undefined);
  const x=run();x.snapshots[5].frames[0].id="different";
  assert.equal(matchingPrevious(x,x.snapshots[5],m),undefined);
  const y=run();selectedValues(y.snapshots[4],c)[0].declaration={file:"main.c",line:4};selectedValues(y.snapshots[5],c)[0].declaration={file:"main.c",line:6};
  assert.equal(matchingPrevious(y,y.snapshots[5],m),undefined);
});
test("stress fixture: current function roots only, stable finite layout and actual sparse indices",()=>{
  const r=loadFixture("data-first-stress").run,l=modules(r);
  for(const s of r.snapshots){
    const v=visibleModules(l,s);
    assert.ok((s.frames[0]?.variables??[]).every(x=>v.some(m=>m.column.name===x.name)));
    assert.ok(v.every(m=>m.column.global||m.column.fn===s.location.function));
    assert.ok(v.every(m=>Number.isFinite(moduleWidth(m.shape))&&moduleWidth(m.shape)>0));
  }
  const sparse=l.find(m=>m.column.name==="sparse")!;
  assert.equal(sparse.shape.children.length,4);assert.ok(sparse.shape.children.some(c=>c.index!>100));
  assert.equal(visibleModules(l,r.snapshots[0]).filter(m=>!m.column.global).length,5);
});
test("stable array compression retains all real indices, protects changed elements, refuses missing values",()=>{
  const r=loadFixture("adaptive-array").run,a=modules(r).find(m=>m.column.name==="a")!;
  const groups=arraySegments(a.shape);
  assert.deepEqual(groups.flat().map(c=>c.index),a.shape.children.map(c=>c.index));
  assert.ok(groups.every(g=>g.length===1||g.every(c=>!c.important)));
  const sample=selectedValues(r.snapshots[0],a.column)[0];
  for(const g of groups.filter(g=>g.length>1)){
    assert.notEqual(foldValue(g,sample,undefined),undefined);
    assert.equal(foldValue(g,{...sample,children:[]},undefined),undefined);
  }
});
test("dense marker path bounded, exact navigation preserved",()=>{
  const ids=Array.from({length:10000},(_,i)=>i);
  assert.equal((markerPath(ids,10000).match(/M/g)??[]).length,1001);
  assert.equal(nextEvent(ids,8999,1),9000);assert.equal(nextEvent(ids,8999,-1),8998);
});
test("sample rules and raw output differences; no success on abnormal run",()=>{
  const d=parseDocument(structuredClone(fixture));d.expectedBytes=new TextEncoder().encode("17 25 30 40\r\n");
  assert.equal(sampleResult(d),"输出一致");assert.equal(outputRows(d)[0].equal,false);
  d.comparison="";assert.equal(sampleResult(d),"输出不一致");
  d.run.reason="cancelled";assert.equal(sampleResult(d),"运行未正常结束");
  d.run.reason="exited";d.run.output.stdout_truncated=true;assert.equal(sampleResult(d),"输出不完整");
});
test("source highlighting preserves text and ignores keywords in identifiers/comments",()=>{
  const t=sourceTokens("int iffy; /* for\nwhile */\nreturn 1;");
  assert.deepEqual(t.map(l=>l.map(x=>x.text).join("")),["int iffy; /* for","while */","return 1;"]);
  assert.deepEqual(t.flat().filter(x=>x.kind==="keyword").map(x=>x.text),["int","return"]);
});

test("shared matrix column widths include later rows",()=>{
  const rows={name:"a",kind:"array",type:"int[][]",children:[{name:"[0]",index:0,kind:"array",type:"int[]",children:[{name:"[0]",index:0,kind:"scalar",type:"int",width:1,children:[]}]},{name:"[1]",index:1,kind:"array",type:"int[]",children:[{name:"[0]",index:0,kind:"scalar",type:"int",width:18,children:[]}]}]};
  assert.equal(sharedFields(rows)[0].width,18);assert.equal(rows.children[0].children[0].width,1);
});


import {samplePreview,coverageNote,sourceMotion} from "./observation.ts";
test("sample preview is single-line, preserves visible line breaks, and bounds long text",()=>{
  assert.equal(samplePreview("2\r\n2 7\n0 5"),"2 ↵ 2 7 ↵ 0 5");
  assert.equal(samplePreview(""),"（空）");assert.equal(samplePreview(null),"未提供");
  assert.ok(!/[\r\n\t]/.test(samplePreview("a\r\nb\tc\nd\r")));
  assert.equal(samplePreview("x".repeat(1000),20).length,21);
});
test("coverage shown only when the grid cannot express missing array coordinates",()=>{
  const r=run(),a=modules(r).find(m=>m.column.name==="a")!;
  assert.equal(coverageNote(a.shape,selectedValues(r.snapshots[0],a.column)[0],true),"");
  const b=loadFixture("adaptive-array").run,m=modules(b).find(m=>m.column.name==="a")!;
  assert.equal(coverageNote(m.shape,selectedValues(b.snapshots[0],m.column)[0],true),"16/100010");
  assert.equal(coverageNote(a.shape,undefined,true),"—");
});
test("source motion follows actual origin line direction, never distant seek or another file",()=>{
  const r=run();
  assert.equal(sourceMotion(r,0,1),0);
  assert.equal(sourceMotion(r,1,2),21);
  assert.equal(sourceMotion(r,2,3),-42);
  assert.equal(sourceMotion(r,3,2),42);
  assert.equal(sourceMotion(r,0,5),0);
  r.snapshots[2].location.file="other.c";
  assert.equal(sourceMotion(r,2,3),0);
});
