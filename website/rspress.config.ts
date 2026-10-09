import * as fs from 'node:fs';
import * as path from 'node:path';
import { defineConfig } from '@rspress/core';
import mermaid from 'rspress-plugin-mermaid';
import sidebar from './sidebar.json' with { type: 'json' };

// The example manifests ship with each build (docs/public), so every docs
// version links its own copy. Pages link them as `assets/<name>.yaml`, which
// the dead-link check does not resolve; accept those links when the file exists.
const examplesAssets = path.join(__dirname, '..', 'docs', 'public', 'volume-provisioning', 'static-provisioning', 'examples', 'assets');
const isExampleManifest = (url: string) => {
  const match = /^assets\/([\w.-]+\.ya?ml)$/.exec(url);
  return match !== null && fs.existsSync(path.join(examplesAssets, match[1]));
};

const repo = 'https://github.com/scality/mountpoint-s3-csi-driver';

export default defineConfig({
  // Replaced at load time by rspress-portable-base, so one build works
  // under every version folder and the latest symlink.
  base: '/REPLACE_ME_PREFIX/',
  root: path.join(__dirname, '..', 'docs'),
  title: 'Scality CSI Driver for S3',
  description: 'Documentation for Scality CSI Driver for S3',
  icon: '/scality-logo.png',
  logo: '/scality-logo.png',
  logoText: 'CSI Driver for S3',
  plugins: [mermaid()],
  markdown: {
    link: { checkDeadLinks: { excludes: isExampleManifest } },
  },
  themeConfig: {
    sidebar: { '/': sidebar },
    socialLinks: [{ icon: 'github', mode: 'link', content: repo }],
    editLink: { docRepoBaseUrl: `${repo}/edit/main/docs/` },
    footer: {
      message: `Licensed under the <a href="${repo}/blob/main/LICENSE">Apache License 2.0</a>`,
    },
  },
});
