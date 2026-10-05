import { Effect } from "effect";
import { greet } from "./greet.ts";

if (import.meta.main) {
  Effect.runSync(greet(Deno.args[0] ?? "world"));
}
