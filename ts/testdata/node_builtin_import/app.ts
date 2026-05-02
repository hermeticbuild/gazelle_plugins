import * as fs from "fs";
import * as path from "node:path";

export const cwd = () => path.resolve(fs.realpathSync("."));
