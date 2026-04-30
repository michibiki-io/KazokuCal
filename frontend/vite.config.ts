import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  plugins: [svelte()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080'
    }
  }
});
