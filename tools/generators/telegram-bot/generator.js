const {
  addProjectConfiguration,
  formatFiles,
  generateFiles,
} = require("@nx/devkit");
const { execSync } = require("child_process");
const path = require("path");

module.exports = async function telegramBotGenerator(tree, options) {
  const name = options.name;
  const moduleName = name.replace(/-/g, "_");
  const description = options.description || `Telegram bot: ${name}`;
  const projectRoot = `apps/${name}`;

  generateFiles(tree, path.join(__dirname, "files"), projectRoot, {
    name,
    moduleName,
    description,
  });

  addProjectConfiguration(tree, name, {
    root: projectRoot,
    projectType: "application",
    sourceRoot: `${projectRoot}/src`,
    tags: [],
    targets: {
      build: {
        executor: "nx:run-commands",
        options: {
          command: "uv sync --locked",
          cwd: projectRoot,
        },
      },
      serve: {
        executor: "nx:run-commands",
        options: {
          command: `uv run python -m ${moduleName}`,
          cwd: projectRoot,
        },
      },
      test: {
        executor: "nx:run-commands",
        cache: true,
        options: {
          command: "uv run pytest",
          cwd: projectRoot,
        },
      },
      lint: {
        executor: "nx:run-commands",
        cache: true,
        options: {
          command: "uv run ruff check . && uv run ruff format --check .",
          cwd: projectRoot,
        },
      },
      "docker-build": {},
      "docker-push": {},
    },
  });

  await formatFiles(tree);

  return () => {
    execSync("uv lock", {
      cwd: path.join(tree.root, projectRoot),
      stdio: "inherit",
    });
  };
};
