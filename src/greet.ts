import { Effect } from "effect";

/** Pure: builds the greeting. Kept separate from I/O so it can be property-tested. */
export const greeting = (name: string): string => `Hello, ${name.trim() || "world"}!`;

/** Effectful: writes the greeting to the console. */
export const greet = (name: string): Effect.Effect<void> => Effect.log(greeting(name));
