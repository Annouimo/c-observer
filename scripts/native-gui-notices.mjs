import {collectNotices} from './collect-notices.mjs';
await collectNotices(process.argv[2],[{GOOS:process.env.GOOS,GOARCH:process.env.GOARCH,CGO_ENABLED:process.env.CGO_ENABLED}],'./gui');
