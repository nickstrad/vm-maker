import { assertEquals } from "@std/assert";
import { Arbitrary, Effect, Either, Schema } from "effect";
import fc from "fast-check";
import { greet, greeting, Name } from "./greeting.ts";

const run = (input: string) => Effect.runSync(Effect.either(greet(input)));

Deno.test("greet trims and greets a valid name", () => {
  assertEquals(run("  Ada "), Either.right("Hello, Ada!"));
});

Deno.test("greet rejects blank input with InvalidName", () => {
  const result = run("   ");
  assertEquals(Either.isLeft(result) && result.left._tag, "InvalidName");
});

Deno.test("property: every schema-valid name round-trips into its greeting", () => {
  fc.assert(
    fc.property(Arbitrary.make(Name), (name) => {
      assertEquals(run(name), Either.right(greeting(name)));
    }),
  );
});

Deno.test("property: greet never throws and only succeeds for decodable input", () => {
  fc.assert(
    fc.property(fc.string({ maxLength: 100 }), (input) => {
      const ok = Either.isRight(Schema.decodeUnknownEither(Name)(input));
      assertEquals(Either.isRight(run(input)), ok);
    }),
  );
});
