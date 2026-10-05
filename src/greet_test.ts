import { assertEquals } from "@std/assert";
import fc from "fast-check";
import { greeting } from "./greet.ts";

Deno.test("greeting defaults to world for blank names", () => {
  assertEquals(greeting(""), "Hello, world!");
  assertEquals(greeting("   "), "Hello, world!");
});

Deno.test("greeting always wraps the trimmed name (property)", () => {
  fc.assert(
    fc.property(fc.string().filter((s) => s.trim().length > 0), (name) => {
      assertEquals(greeting(name), `Hello, ${name.trim()}!`);
    }),
  );
});
