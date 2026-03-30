/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Claude-style color palette
        background: '#ffffff',
        'background-dark': '#1e1e1e',
        surface: '#f8f9fa',
        'surface-dark': '#2d2d2d',
        border: '#e5e7eb',
        'border-dark': '#404040',
        primary: '#10a37f',
        'primary-dark': '#1a7f64',
        text: '#1f2937',
        'text-dark': '#f3f4f6',
        'text-muted': '#6b7280',
        'text-muted-dark': '#9ca3af',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      boxShadow: {
        'soft': '0 2px 8px rgba(0, 0, 0, 0.08)',
        'soft-dark': '0 2px 8px rgba(0, 0, 0, 0.3)',
      },
    },
  },
  plugins: [],
}