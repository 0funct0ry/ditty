import type { Config } from "tailwindcss";

// Palette and type scale carried over from internal-docs/MOCKUP.html so
// chrome (M5) and the settings drawer/themes (M6) compose from the same
// named values instead of re-deriving colors.
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#171B24",
        "ink-deep": "#0E1116",
        "ink-soft": "#212734",
        line: "#2C3342",
        paper: "#ECEDE8",
        pine: "#3D9C87",
        "pine-deep": "#2C7A6B",
        saffron: "#E0A32E",
        rust: "#C4634C",
        muted: "#7C8492",
        "muted-bright": "#A6ADB9",
      },
      fontFamily: {
        sans: ["Bricolage Grotesque", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
      spacing: {
        chrome: "44px",
        status: "26px",
      },
    },
  },
  plugins: [],
} satisfies Config;
