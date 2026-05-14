const fs = require("fs");
const React = require("react");

require("@scope/pkg/subpath");

module.exports = require("lodash/fp");

const packageName = "ignored";
require(packageName);
require(`ignored`);
require.resolve("ignored");

function local(require: (path: string) => unknown) {
  require("ignored");
}
