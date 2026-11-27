import fs from 'node:fs/promises';
import path from 'node:path';

const dir=path.resolve('gui/frontend');
const list=JSON.parse(await fs.readFile(path.join(dir,'dist/bundled-packages.json'),'utf8'));
const sections=['# Bundled frontend dependencies',''];
for(const name of list){
  if(!/^(?:@[\w.-]+\/)?[\w.-]+$/.test(name))throw Error('Invalid package name: '+name);
  const root=path.join(dir,'node_modules',name);
  const pkg=JSON.parse(await fs.readFile(path.join(root,'package.json'),'utf8'));
  const files=(await fs.readdir(root)).filter(n=>/^(?:licen[sc]es?|copying|copyright|notice|unlicense)(?:$|[._-])/i.test(n)).sort();
  if(!files.some(n=>/^(?:licen[sc]es?|copying|unlicense)(?:$|[._-])/i.test(n)))throw Error('Missing upstream license: '+name);
  sections.push('## '+pkg.name+' '+pkg.version,'');
  for(const file of files)sections.push('--- '+file+' ---',(await fs.readFile(path.join(root,file),'utf8')).trimEnd(),'');
}
await fs.writeFile(process.argv[2],sections.join('\n')+'\n');
