import { Data, Effect, Schema } from "effect";

/** A name we are willing to greet: trimmed, non-empty, and bounded in length. */
export const Name = Schema.Trim.pipe(
  Schema.nonEmptyString(),
  Schema.maxLength(64),
  Schema.brand("Name"),
);
export type Name = typeof Name.Type;

export class InvalidName extends Data.TaggedError("InvalidName")<{ readonly input: string }> {}

/** Pure: formats a greeting for an already-validated name. */
export const greeting = (name: Name): string => `Hello, ${name}!`;

/** Decodes untrusted input, failing with a typed error instead of throwing. */
export const greet = (input: string): Effect.Effect<string, InvalidName> =>
  Schema.decodeUnknown(Name)(input).pipe(
    Effect.map(greeting),
    Effect.mapError(() => new InvalidName({ input })),
  );
