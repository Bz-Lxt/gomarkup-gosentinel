/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{vue,ts}"],
  theme: {
    extend: {
      colors: {
        ink: "#070b14",
        panel: "#0d1524",
        line: "#1a2740",
        cyan: "#3ee0c8",
        amber: "#f5b942",
        rose: "#ff5c7a",
        mint: "#5ee6a0",
        fog: "#9bb0c9",
      },
      fontFamily: {
        sans: ["Outfit", "system-ui", "sans-serif"],
        mono: ["IBM Plex Mono", "ui-monospace", "monospace"],
      },
      boxShadow: {
        glow: "0 0 40px rgba(62, 224, 200, 0.12)",
      },
    },
  },
  plugins: [],
};
