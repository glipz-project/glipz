import typography from "@tailwindcss/typography";

/** @type {import('tailwindcss').Config} */
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{vue,js,ts}"],
  theme: {
    extend: {
      colors: {
        frame: "rgb(var(--color-frame) / <alpha-value>)",
        neutral: {
          50: 'rgb(var(--theme-surface-muted) / <alpha-value>)',
          100: 'rgb(var(--theme-surface-muted) / <alpha-value>)',
          200: 'rgb(var(--color-frame) / <alpha-value>)',
          300: 'rgb(var(--color-frame) / <alpha-value>)',
          400: 'rgb(var(--theme-text-muted) / <alpha-value>)',
          500: 'rgb(var(--theme-text-muted) / <alpha-value>)',
          600: 'rgb(var(--theme-text-muted) / <alpha-value>)',
          700: 'rgb(var(--theme-text) / <alpha-value>)',
          800: 'rgb(var(--theme-text) / <alpha-value>)',
          900: 'rgb(var(--theme-text) / <alpha-value>)',
          950: 'rgb(var(--theme-surface) / <alpha-value>)',
        },
        lime: {
          50: 'rgb(var(--theme-accent-bg) / <alpha-value>)',
          100: 'rgb(var(--theme-accent-bg) / <alpha-value>)',
          200: 'rgb(var(--theme-accent-bg) / <alpha-value>)',
          300: 'rgb(var(--theme-accent-bg) / <alpha-value>)',
          400: 'rgb(var(--theme-accent-text) / <alpha-value>)',
          500: 'rgb(var(--theme-accent-bg-strong) / <alpha-value>)',
          600: 'rgb(var(--theme-accent-bg-strong) / <alpha-value>)',
          700: 'rgb(var(--theme-accent-text) / <alpha-value>)',
          800: 'rgb(var(--theme-accent-text-strong) / <alpha-value>)',
          900: 'rgb(var(--theme-accent-text-strong) / <alpha-value>)',
          950: 'rgb(var(--theme-accent-bg) / <alpha-value>)',
        },
      },
      fontFamily: {
        sans: ["Inter", "Noto Sans JP", "system-ui", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "sans-serif"],
      },
    },
  },
  plugins: [typography],
};
