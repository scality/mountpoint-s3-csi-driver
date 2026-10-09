import * as path from 'node:path';
import { defineConfig } from '@rspress/core';
import mermaid from 'rspress-plugin-mermaid';
import sidebar from './sidebar.json' with { type: 'json' };

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
    link: { checkDeadLinks: true },
  },
  themeConfig: {
    sidebar: { '/': sidebar },
    socialLinks: [{ icon: 'github', mode: 'link', content: repo }],
    editLink: { docRepoBaseUrl: `${repo}/edit/main/docs/` },
    footer: {
      message: `Copyright © 2025 Scality, Inc. Licensed under the <a href="${repo}/blob/main/LICENSE">Apache License 2.0</a>`,
    },
  },
});
