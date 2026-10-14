// SPDX-License-Identifier: Apache-2.0
import { copyFileSync, cpSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

const site = resolve(import.meta.dirname, '..');
const publicDir = resolve(site, 'public');
mkdirSync(publicDir, { recursive: true });
for (const file of [
  'CNAME', 'demo.txt', 'demo.gif', 'demo.mp4', 'demo-poster.jpg',
]) copyFileSync(resolve(site, file), resolve(publicDir, file));
cpSync(resolve(site, 'fonts'), resolve(publicDir, 'fonts'), { recursive: true, force: true });
