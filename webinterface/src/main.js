// frontend/src/main.js
import App from './app.svelte';
import './global.css'; // Import Tailwind CSS

const app = new App({
  target: document.body,
});

export default app;
