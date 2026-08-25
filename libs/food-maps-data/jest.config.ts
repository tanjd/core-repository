module.exports = {
  displayName: "food-maps-data",
  preset: "../../jest.preset.js",
  testEnvironment: "node",
  moduleFileExtensions: ["ts", "js", "html"],
  coverageDirectory: "../../coverage/libs/food-maps-data",
  transformIgnorePatterns: ["node_modules/(?!sql\\.js)"],
};
