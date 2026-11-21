// Regular expression to match lone UTF-16 surrogates (unpaired high or low surrogates)
const LONE_SURROGATE = /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/;

/**
 * Checks if a string is valid storable text.
 * @param value The string to check for storable text validity.
 * @returns True if the string is storable text (does not contain null characters or lone UTF-16 surrogates), false otherwise.
 */
export function isStorableText(value: string): boolean {
    return !value.includes('\u0000') && !LONE_SURROGATE.test(value);
}