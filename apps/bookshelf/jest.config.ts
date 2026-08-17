module.exports = {
  displayName: "bookshelf",
  preset: "../../jest.preset.js",
  transform: {
    "^.+\\.[tj]sx?$": [
      "@swc/jest",
      {
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
  moduleFileExtensions: ["ts", "tsx", "js", "jsx"],
  moduleNameMapper: {
    "^@/(.*)$": "<rootDir>/src/$1",
  },
  coverageDirectory: "../../coverage/apps/bookshelf",
  setupFilesAfterEnv: ["<rootDir>/src/test-setup.ts"],
  testEnvironment: "jest-environment-jsdom",
};
