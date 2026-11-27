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
  const y=structuredClone(fixture);delete y.synthetic;y.format="c-observer-record";y.schema_version=1;y.run.session.backend="gdb-mi";y.stdin="";y.expected_stdout=null;y.observer_version="dev";y.recorded_at="2026-11-27T00:00:00Z";y.platform="darwin";y.arch="arm64";y.run.snapshots=null;y.run.session.sources=null;
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


import {trackPosition,markerHit,navigationTarget,pulseTargets,statusHint} from "./observation.ts";
test("timeline hit tests real events, deterministic ties, clamp and empty record",()=>{
  assert.equal(trackPosition(50,100,101),50);
  assert.equal(trackPosition(-20,100,101),0);
  assert.equal(trackPosition(130,100,101),100);
  assert.equal(trackPosition(50,100,0),0);
  assert.equal(markerHit([10,20],15,101,100),10);
  assert.equal(markerHit([10,20],18,101,100),20);
  assert.equal(markerHit([10,20],50,101,100),undefined);
  assert.equal(markerHit([],15,101,100),undefined);
  const dense=Array.from({length:10000},(_,i)=>i);
  assert.equal(markerHit(dense,7894.4,10000,300),7894);
});
test("keyboard navigation retains exact original events",()=>{
  assert.equal(navigationTarget("ArrowLeft",true,5,10,[2,5,8]),2);
  assert.equal(navigationTarget("ArrowRight",true,5,10,[2,5,8]),8);
  assert.equal(navigationTarget("ArrowRight",true,8,10,[2,5,8]),8);
  assert.equal(navigationTarget("ArrowLeft",false,0,10,[]),0);
  assert.equal(navigationTarget("End",false,0,10,[]),9);
});
test("flash targets compare only adjacent actual leaf values in either direction",()=>{
  const r=run(),c=member(member({name:"a",fn:"main"},{name:"[1]",index:1}),{name:"score"});
  assert.deepEqual([...pulseTargets(r,4,5)],[columnKey(c)]);
  assert.deepEqual([...pulseTargets(r,5,4)],[columnKey(c)]);
  assert.equal(pulseTargets(r,0,1).size,0);
  assert.equal(pulseTargets(r,0,5).size,0);
  assert.equal(pulseTargets(r,3,4,1).size,0);
  assert.equal(statusHint(undefined,true),"当前观察无对应对象");
  assert.equal(statusHint(undefined,false),"当前观察未记录该成员");
});


import {replayFeedback} from "./observation.ts";
test("normal scrub uses the same semantic motion as step navigation; rapid scrub only suppresses pulses",()=>{
  const r=run();
  assert.equal(replayFeedback(r,1,2,"drag",200).offset,21);
  assert.equal(replayFeedback(r,2,3,"drag",20).offset,-42);
  assert.equal(replayFeedback(r,2,3,"drag",20).duration,85);
  assert.equal(replayFeedback(r,4,5,"drag",200).pulses.size,1);
  assert.equal(replayFeedback(r,4,5,"drag",20).pulses.size,0);
  assert.equal(replayFeedback(r,5,4,"direct",0).pulses.size,1);
  assert.equal(replayFeedback(r,1,3,"direct",0).offset,-21);
  assert.equal(replayFeedback(r,0,5,"drag",200).offset,0);
  assert.equal(replayFeedback(r,1,2,"seek",200).offset,0);
  assert.equal(replayFeedback(r,4,5,"seek",200).pulses.size,0);
});

test("live Go record envelope accepts nil slices and rejects invalid offsets",()=>{
 const d={format:"c-observer-record",schema_version:1,observer_version:"dev",recorded_at:"2026-11-27T00:00:00Z",platform:"darwin",arch:"arm64",stdin:"NQo=",expected_stdout:"",comparison:"",run:{reason:"exited",session:{backend:"lldb-dap",sources:null},snapshots:[{id:0,location:{file:"",line:0},frames:null,output_offsets:{stdout_bytes:0,stderr_bytes:0}}],output:{stdout:"",stderr:null}}};
 const p=parseDocument(structuredClone(d));assert.equal(p.stdin,"5\n");assert.equal(p.expected,"");assert.equal(p.synthetic,false);
 d.run.snapshots[0].output_offsets.stdout_bytes=1;assert.throws(()=>parseDocument(d));
});


import {charByte,stringSummary,compactValue,hasDetail,detailRoot,previewChildren,type Variable,type Shape} from "./observation.ts";
test("character buffer summary uses only contiguous recorded bytes and real terminator",()=>{
  const c=(i:number,value:string):Variable=>({name:"["+i+"]",index:i,type:"char",kind:"scalar",status:"observed",value});
  const v:Variable={name:"v",type:"char[10]",kind:"string",status:"observed",array_length:10,children:[c(0,"67 'C'"),c(1,"'E'"),c(2,"'\\0'"),c(3,"'\\x81'")]};
  assert.deepEqual(stringSummary(v),{text:'"CE"',end:2,complete:true});
  assert.equal(charByte(c(3,"'\\x81'")),129);
  assert.equal(charByte(c(3,"-1 '\\377'")),255);
  assert.equal(stringSummary({...v,children:[c(0,"'C'"),c(2,"'\\0'")]})?.complete,false);
  assert.equal(stringSummary({...v,children:[c(0,"'C'"),c(1,"'E'")]})?.text,'"CE"…');
});
test("detail follows root focus and keeps parent context for selected child",()=>{
  const r=loadFixture("adaptive-array").run,l=modules(r),m=l.find(x=>x.column.name==="a")!;
  assert.ok(hasDetail(m.shape));
  assert.deepEqual(detailRoot(l,member(m.column,m.shape.children[0])),m);
  assert.equal(detailRoot(l,null),undefined);
  assert.ok(previewChildren(m.shape).length<=8);
  assert.equal(m.shape.children.length,16);
  assert.equal(detailRoot(modules(run()),{name:"i",fn:"main"}),undefined);
});
test("typed pointer address changes count even with children, never match old target member across addresses",()=>{
  const r=run(),c={name:"ptr",fn:"main"},ch={name:"score",type:"int",kind:"scalar",status:"observed",value:"7"};
  const ptr=(value:string):Variable=>({name:"ptr",type:"Team *",kind:"pointer",status:"observed",value,children:[ch]});
  r.snapshots[0].frames[0].variables.push(ptr("0x10"));
  r.snapshots[1].frames[0].variables.push(ptr("0x20"));
  assert.equal(compactValue(ptr("0x10")),"0x10");
  assert.deepEqual(changeEvents(r,c),[1]);
  assert.equal(matchingPrevious(r,r.snapshots[1],member(c,ch)),undefined);
});


import {previousChildren} from "./observation.ts";
test("even short compressed arrays have a detail route and pointer children never compare across addresses",()=>{
  const shape:Shape={name:"a",kind:"array",type:"int[4]",children:Array.from({length:4},(_,i)=>({name:"["+i+"]",index:i,kind:"scalar",type:"int",constant:"0",children:[]}))};
  assert.equal(hasDetail(shape),true);
  const p:Variable={name:"p",kind:"pointer",type:"Team *",status:"observed",value:"0x10"};
  assert.equal(previousChildren(p,{...p,value:"0x20"}),undefined);
  assert.equal(previousChildren(p,{...p}),p);
});


import {isInline,shapeLabel,compoundSummary,tableFields,fieldWidth} from "./observation.ts";
test("nested course data stays shallow and table-shaped; detail follows the actual path",()=>{
  const leaf=(name:string,width=3):Shape=>({name,type:"int",kind:"scalar",children:[],width});
  const str:Shape={name:"name",type:"char[20]",kind:"string",arrayLength:20,width:7,children:[{...leaf("[0]"),index:0}]};
  const q:Shape={name:"question",type:"Question[20]",kind:"array",arrayLength:20,children:Array.from({length:20},(_,i)=>({name:"["+i+"]",index:i,type:"Question",kind:"struct",children:[leaf("AC_time"),leaf("WA_count")]}))};
  const row:Shape={name:"[0]",index:0,type:"Team",kind:"struct",children:[leaf("id"),str,leaf("AC_count"),leaf("punish"),q]};
  const a:Shape={name:"teams",type:"Team[200010]",kind:"array",arrayLength:200010,children:Array.from({length:5},(_,i)=>({...row,index:i,name:"["+i+"]"}))};
  const x:Shape={...row,name:"x",index:undefined,kind:"pointer",type:"Team *"};
  assert.deepEqual(tableFields(a).map(c=>c.name),["id","name","AC_count","punish","question"]);
  assert.ok(moduleWidth(x)<=460);
  assert.ok(moduleWidth(a)<850);
  assert.ok(isInline(str));assert.equal(shapeLabel(str),"name[20]");
  assert.ok(moduleWidth({...leaf("compare_result"),width:3})>=160);
  assert.ok(fieldWidth(q)<200);
  const c={name:"x",fn:"compare"},l=[{column:c,shape:x}];
  const cq=member(c,q),item=member(cq,q.children[2]),score=member(item,q.children[2].children[0]);
  assert.deepEqual(detailRoot(l,cq),{column:cq,shape:q});
  assert.deepEqual(detailRoot(l,item),{column:item,shape:q.children[2]});
  assert.deepEqual(detailRoot(l,score),{column:item,shape:q.children[2]});
  assert.deepEqual(detailRoot(l,member(c,str)),{column:member(c,str),shape:str});
  assert.equal(compoundSummary(q,undefined),"?");
  const v:Variable={name:"question",kind:"array",type:"Question[20]",status:"observed",array_length:20,children:[],children_complete:false};
  assert.equal(compoundSummary(q,v),"[20] · 0");
  assert.equal(previewChildren(q).length,6);
});
