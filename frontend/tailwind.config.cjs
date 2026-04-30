module.exports = {
  content: [
    './index.html',
    './src/**/*.{svelte,ts,js}',
    './node_modules/flowbite-svelte/**/*.{html,js,svelte,ts}',
    './node_modules/flowbite/**/*.{js,ts}'
  ],
  theme: {
    extend: {}
  },
  plugins: [require('flowbite/plugin')]
};
