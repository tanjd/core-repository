const nxPreset = require("@nx/jest/preset").default;

// Replace the preset's default `ts-jest` transform with `@swc/jest` for every
// project in this workspace. `swcrc: false` is set here (not per-project)
// because libs with an on-disk `.swcrc` for their SWC build target (e.g.
// libs/food-maps-data) exclude spec/test files from that build — which
// @swc/jest would otherwise pick up and refuse to transform. Ignoring
// `.swcrc` at the Jest layer keeps this a non-issue for every project,
// whether or not they have one.
module.exports = {
  ...nxPreset,
  transform: {
    "^.+\\.[tj]sx?$": [
      "@swc/jest",
      {
        swcrc: false,
        jsc: {
          transform: {
            react: {
              runtime: "automatic",
            },
          },
          target: "es2022",
        },
      },
    ],
  },
};
