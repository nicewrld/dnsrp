// frontend/tailwind.config.js
module.exports = {
  darkMode: 'class',
  content: ['./src/**/*.svelte', './public/index.html'],
  theme: {
    extend: {
      colors: {
        primary: '#a3bffa',    // Pastel Blue
        secondary: '#f7aef8',  // Pastel Pink
        accent: '#a9f7ae',     // Pastel Green
        background: '#121212', // Dark Background
        text: '#e0e0e0',       // Light Text
      },
    },
  },
  plugins: [],
};
