import { assert, assertEquals } from "@std/assert";
import { Effect, Result, Schema } from "effect";
import fc from "fast-check";
import { greet, Name } from "./greeting.ts";

const run = (input: string) => Effect.runSync(Effect.result(greet(input)));

/** Names the schema should accept: trimmed, 1..64 chars, padded with optional whitespace. */
const validName = fc
  .string({ minLength: 1, maxLength: 64 })
  .filter((s) => s.trim() === s && s.length > 0);
const padding = fc.constantFrom("", " ", "\t", "  \n");

Deno.test("greet trims and greets a valid name", () => {
  assertEquals(run("  Ada "), Result.succeed("Hello, Ada!"));
});

Deno.test("greet rejects blank input with InvalidName", () => {
  const result = run("   ");
  assert(Result.isFailure(result));
  assertEquals(result.failure._tag, "InvalidName");
});

Deno.test("property: surrounding whitespace never changes the greeting", () => {
  fc.assert(
    fc.property(validName, padding, padding, (name, left, right) => {
      assertEquals(run(left + name + right), Result.succeed(`Hello, ${name}!`));
    }),
  );
});

Deno.test("property: greet never throws and succeeds exactly when Name decodes", () => {
  fc.assert(
    fc.property(fc.string({ maxLength: 100 }), (input) => {
      const decodes = Result.isSuccess(Schema.decodeUnknownResult(Name)(input));
      assertEquals(Result.isSuccess(run(input)), decodes);
    }),
  );
});

Deno.test("property: names longer than 64 chars after trimming are rejected", () => {
  fc.assert(
    fc.property(
      fc.string({ minLength: 65, maxLength: 200 }).filter((s) => s.trim().length > 64),
      (s) => {
        assert(Result.isFailure(run(s)));
      },
    ),
  );
});
