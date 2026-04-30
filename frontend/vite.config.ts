import { svelte } from '@sveltejs/vite-plugin-svelte';
import { execSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { defineConfig } from 'vite';

type PackageJson = {
  version?: string;
};

const packageJson = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')) as PackageJson;

function gitOutput(args: string[]): string {
  try {
    return execSync(['git', ...args].join(' '), { encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function envValue(name: string): string | undefined {
  const value = process.env[name]?.trim();
  return value || undefined;
}

function githubRepository(): string {
  const repository = envValue('GITHUB_REPOSITORY');
  if (repository) return repository;

  const remote = gitOutput(['remote', 'get-url', 'origin']);
  const match = remote.match(/github\.com[:/]([^/]+\/[^/.]+)(?:\.git)?$/);
  return match?.[1] ?? 'michibiki-io/KazokuCal';
}

const appVersion = envValue('VITE_BUILD_VERSION') ?? envValue('BUILD_VERSION') ?? packageJson.version ?? '';
const releaseCommit = envValue('VITE_BUILD_COMMIT') ?? envValue('BUILD_COMMIT') ?? gitOutput(['rev-parse', 'HEAD']);

export default defineConfig({
  base: './',
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
    __GITHUB_REPOSITORY__: JSON.stringify(githubRepository()),
    __RELEASE_COMMIT__: JSON.stringify(releaseCommit)
  },
  plugins: [svelte()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080'
    }
  }
});
