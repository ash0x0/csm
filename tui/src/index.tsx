import React from 'react';
import { render } from 'ink';
import { AppWithInput } from './App.js';
import { init as initCsm } from './csm.js';

// Args passed by Go: node index.js --csm-bin /path/to/csm --claude-dir /home/user/.claude
const args = process.argv.slice(2);
let csmBin = 'csm';
let claudeDir = '';

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--csm-bin' && args[i + 1]) csmBin = args[++i];
  if (args[i] === '--claude-dir' && args[i + 1]) claudeDir = args[++i];
}

initCsm(csmBin, claudeDir);

render(<AppWithInput />, {
  alternateScreen: true,
  exitOnCtrlC: true,
});
