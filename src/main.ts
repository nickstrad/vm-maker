import { Effect } from "effect";
import { greet } from "./hello/greeting.ts";

const program = (input: string) =>
  greet(input).pipe(
    Effect.flatMap(Effect.log),
    Effect.catchTag("InvalidName", (e) =>
      Effect.logError(`cannot greet ${JSON.stringify(e.input)}`).pipe(
        Effect.zipRight(Effect.sync(() =>
          Deno.exitCode = 2
        )),
      )),
  );

if (import.meta.main) {
  Effect.runSync(program(Deno.args[0] ?? "world"));
}
