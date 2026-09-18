const nextJest = require("next/jest");

const createJestConfig = nextJest({ dir: "./" });

const customJestConfig = {
  testEnvironment: "node",
  moduleNameMapper: {
    "^@/(.*)$": "<rootDir>/src/$1",
  },
  // .next/standalone ships its own copy of package.json (from next.config.js's
  // output: "standalone"), which otherwise collides with the root one.
  modulePathIgnorePatterns: ["<rootDir>/.next/"],
};

module.exports = createJestConfig(customJestConfig);
