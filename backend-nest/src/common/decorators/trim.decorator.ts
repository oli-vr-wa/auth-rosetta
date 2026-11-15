import { Transform, type TransformFnParams } from "class-transformer";

/**
 * A decorator that trims whitespace from string properties in DTOs.
 */
export const Trim = () =>
    Transform(({ value }: TransformFnParams): unknown =>
        typeof value === 'string' ? value.trim() : value,
    );