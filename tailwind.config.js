/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./web/templates/**/*.html"],
  theme: {
    extend: {
      colors: {
        notion: {
          50: "#fafafa",
          100: "#f0f0f0",
          200: "#e4e4e4",
          300: "#d1d1d1",
          400: "#b4b4b4",
          500: "#9a9a9a",
          600: "#818181",
          700: "#6a6a6a",
          800: "#555555",
          900: "#2f2f2f",
        },
      },
    },
  },
  plugins: [],
};
