import nx from "@nx/eslint-plugin";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";

export default [
  ...nx.configs["flat/base"],
  ...nx.configs["flat/javascript"],
  react.configs.flat.recommended,
  react.configs.flat["jsx-runtime"],
  {
    plugins: { "react-hooks": reactHooks },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    settings: { react: { version: "19.1.1" } },
  },
  {
    files: ["**/*.js", "**/*.jsx"],
    rules: {},
  },
];
