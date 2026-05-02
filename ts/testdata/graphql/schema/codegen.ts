// graphql-codegen config. Paths use __dirname so they resolve to wherever
// the file lives in the Bazel sandbox (which differs from the source tree).
import { join } from 'node:path';
import type { CodegenConfig } from '@graphql-codegen/cli';

const here = (rel: string): string => join(__dirname, rel);

const config: CodegenConfig = {
  schema: here('schema.graphql'),
  documents: [here('queries.graphql')],
  generates: {
    [here('generated.ts')]: {
      plugins: ['typescript', 'typescript-operations'],
      config: {
        enumsAsTypes: true,
        avoidOptionals: true,
      },
    },
  },
};

export default config;
