// Assemble previously built, native-tested GUI applications. No cross-builds,
// no lifecycle installation scripts and no dependency on Node during use.
import fs from 'node:fs/promises';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';

const [version, inputPath, outputPath, mode='DRY_RUN']=process.argv.slice(2);
const stable=/^\d+\.\d+\.\d+$/.test(version??'');
const preview=/^\d+\.\d+\.\d+-(?:alpha|beta|rc|dev)\.\d+$/.test(version??'');
if(!['DRY_RUN','PUBLISH_NEXT','PUBLISH_LATEST'].includes(mode)||!stable&&!preview||mode==='PUBLISH_NEXT'&&!preview||mode==='PUBLISH_LATEST'&&!stable)throw Error('Invalid release mode/version');
const input=path.resolve(inputPath),out=path.resolve(outputPath),pkg=path.join(out,'npm');
await fs.mkdir(out,{recursive:true});await fs.mkdir(pkg);
await fs.copyFile('LICENSE',path.join(pkg,'LICENSE'));
const notes=await Promise.all(['frontend','darwin-arm64','win32-x64'].map(x=>fs.readFile(path.join(input,x,'NOTICES.txt'),'utf8')));
if(notes.some(n=>!n.trim()))throw Error('Missing notices');
await fs.writeFile(path.join(pkg,'THIRD_PARTY_NOTICES.txt'),notes.join('\n\n'));
const manifest={schemaVersion:1,version,sessionSchemaVersions:[1],platforms:{}};
for(const [target,relative] of [
 ['darwin-arm64','bin/darwin-arm64/C Observer.app/Contents/MacOS/c-observer'],
 ['win32-x64','bin/win32-x64/c-observer.exe']
]){
 const stage=path.join(out,'standalone-'+target);
 await fs.mkdir(stage);
 // Native archives preserve the executable bits and signed .app contents.
 execFileSync('unzip',['-q',path.join(input,target,'native.zip'),'-d',stage],{stdio:'inherit'});
 const entry=target==='darwin-arm64'?'C Observer.app/Contents/MacOS/c-observer':'c-observer.exe';
 const native=path.join(stage,entry);
 if(!(await fs.stat(native)).isFile())throw Error('Missing binary '+target);
 await fs.chmod(native,0o755);
 await fs.mkdir(path.join(pkg,'bin',target),{recursive:true});
 for(const name of await fs.readdir(stage))await fs.cp(path.join(stage,name),path.join(pkg,'bin',target,name),{recursive:true,preserveTimestamps:true});
 const sha256=createHash('sha256').update(await fs.readFile(path.join(pkg,relative))).digest('hex');
 manifest.platforms[target]={executable:relative,sha256,status:stable?'stable':'preview'};
 for(const name of ['LICENSE','THIRD_PARTY_NOTICES.txt'])await fs.copyFile(path.join(pkg,name),path.join(stage,name));
 await fs.copyFile('docs/npm-readme.md',path.join(stage,'README.md'));
 const archive=path.join(out,'c-observer-'+version+'-'+target+'.zip');
 execFileSync('zip',['-qry',archive,'.'],{cwd:stage,stdio:'inherit'});
}
await fs.writeFile(path.join(pkg,'manifest.json'),JSON.stringify(manifest,null,2)+'\n');
await fs.copyFile('docs/npm-readme.md',path.join(pkg,'README.md'));
await fs.writeFile(path.join(pkg,'package.json'),JSON.stringify({
 name:'@routilet/c-observer',version,description:'C Observer offline graphical C execution observer',
 repository:{type:'git',url:'git+https://github.com/Routilet/c-observer.git'},
 homepage:'https://github.com/Routilet/c-observer#readme',bugs:{url:'https://github.com/Routilet/c-observer/issues'},license:'MIT',
 ...(mode==='DRY_RUN'?{private:true}:{publishConfig:{access:'public',tag:stable?'latest':'next'}}),
 files:['bin','manifest.json','README.md','LICENSE','THIRD_PARTY_NOTICES.txt'],
 exports:{'./manifest.json':'./manifest.json','./package.json':'./package.json'}
},null,2)+'\n');
execFileSync('npm',['pack','--ignore-scripts','--pack-destination',out],{cwd:pkg,stdio:'inherit'});
