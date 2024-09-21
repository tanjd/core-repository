import { ExecutorContext } from "@nx/devkit";

import executor from "./executor";
import { ServeExecutorSchema } from "./schema";

const options: ServeExecutorSchema = {
  textToEcho: "",
};
const context: ExecutorContext = {
  root: "",
  cwd: process.cwd(),
  isVerbose: false,
};

describe("Serve Executor", () => {
  it("can run", async () => {
    const output = await executor(options, context);
    expect(output.success).toBe(true);
  });
});
