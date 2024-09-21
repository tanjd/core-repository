import { PromiseExecutor } from "@nx/devkit";
import { exec } from "child_process";
import { promisify } from "util";

import { ServeExecutorSchema } from "./schema";

export interface EchoExecutorOptions {
  textToEcho: string;
}

const runExecutor: PromiseExecutor<ServeExecutorSchema> = async (options) => {
  console.log("Executor ran for Serve", options);
  console.info(`Options: ${JSON.stringify(options, null, 2)}`);

  const { stdout, stderr } = await promisify(exec)(
    `echo ${options.textToEcho}`,
  );
  console.log(stdout);
  console.error(stderr);
  return {
    success: true,
  };
};

export default runExecutor;
