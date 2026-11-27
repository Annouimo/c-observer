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
