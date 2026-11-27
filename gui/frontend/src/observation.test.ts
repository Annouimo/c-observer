import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseDocument, tokenize, groupTrace, flatten, values, changed, label, valueChanged, type Run } from "./observation.ts";
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
