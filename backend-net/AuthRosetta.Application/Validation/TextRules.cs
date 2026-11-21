namespace AuthRosetta.Application.Validation;

public static class TextRules
{
    /// <summary>
    /// Determines if the given string can be safely stored, ensuring it contains no null characters and all surrogate pairs are valid.
    /// </summary>
    /// <param name="value">The string to check for storable validity.</param>
    /// <returns>True if the string is storable, false otherwise.</returns>
    public static bool IsStorable(string value)
    {
        if (value.Contains('\0')) return false;

        for (int i = 0; i < value.Length; i++)
        {
            if (char.IsHighSurrogate(value[i]))
            {
                if (i + 1 < value.Length && char.IsLowSurrogate(value[i + 1]))
                {
                    i++; // skip the low surrogate
                    continue; // valid surrogate pair, continue checking
                }
                return false; // unpaired high surrogate
            }
            if (char.IsLowSurrogate(value[i])) 
            {
                return false; // unpaired low surrogate
            }
        }
        return true; // all characters are valid
    }
}