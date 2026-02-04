module.exports = {
  "**/*.{json,yml,yaml,md}": (files) => files.map((f) => `just lint-file "${f}"`),
  "**/[Jj]ustfile": () => "just lint justfile",
};
