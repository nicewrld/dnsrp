// frontend/rollup.config.js
import svelte from 'rollup-plugin-svelte';
import resolve from '@rollup/plugin-node-resolve'; // Updated import
import commonjs from '@rollup/plugin-commonjs';    // Updated import
import livereload from 'rollup-plugin-livereload';
import { terser } from 'rollup-plugin-terser';
import sveltePreprocess from 'svelte-preprocess';
import postcss from 'rollup-plugin-postcss';       // Added plugin

const production = !process.env.ROLLUP_WATCH;

export default {
  input: 'src/main.js',
  output: {
    sourcemap: !production,
    format: 'iife',
    name: 'app',
    file: './public/build/bundle.js', // Output to the Go server's public directory
  },
  plugins: [
    svelte({
      preprocess: sveltePreprocess({
        sourceMap: !production,
        postcss: true,
      }),
      compilerOptions: {
        dev: !production,   // Moved 'dev' under 'compilerOptions'
      },
    }),
    postcss({
      extract: true,
      minimize: production,
      sourceMap: !production,
    }),
    resolve({
      browser: true,
      dedupe: ['svelte'],
    }),
    commonjs(),
    !production && livereload('./public'),
    production && terser(),
  ],
  watch: {
    clearScreen: false,
  },
};
