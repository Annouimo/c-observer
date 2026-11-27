// Wails' embedded runtime is served locally by the native asset server.
// Keep the replay-only browser development mode independent of Wails.
type Runtime = {Call:{ByName(name:string,...args:unknown[]):Promise<unknown>}};
let runtime:Runtime|undefined;
export async function connect():Promise<boolean>{
  try{
    const path="/wails/runtime.js";
    runtime=await import(/* @vite-ignore */ path) as Runtime;
    return typeof runtime?.Call?.ByName==="function";
  }catch{return false;}
}
export async function call<T>(method:string,...args:unknown[]):Promise<T>{
  if(!runtime)throw Error("未连接本地 Observer");
  return await runtime.Call.ByName("main.Desktop."+method,...args) as T;
}
export type Initial={Mode:"setup"|"session"|"replay";Source:string;Record:string;Error:string};
