module.exports = {
  displayName: "food-maps",
  preset: "../../jest.preset.js",
  moduleFileExtensions: ["ts", "tsx", "js", "jsx"],
  coverageDirectory: "../../coverage/apps/food-maps",
  setupFilesAfterEnv: ["<rootDir>/src/test-setup.ts"],
  testEnvironment: "jest-environment-jsdom",
};
