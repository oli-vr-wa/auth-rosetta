using System.Text;

namespace AuthRosetta.Application.Validation;

public static class RegisterRules
{
    public const int MaxEmailLength = 256;
    public const int MinPasswordLength = 8;
    public const int MaxPasswordBytes = 72;

    public static void EnsureValid(string? email, string? password)
    {
        var errors = new List<string>();

        var trimmedEmail = email?.Trim();
        if (string.IsNullOrEmpty(trimmedEmail)) 
            errors.Add("Email is required");
        else if (trimmedEmail.Length > MaxEmailLength) 
            errors.Add($"Email must be at most {MaxEmailLength} characters");
        
        if (password is null || password.Length < MinPasswordLength)
            errors.Add($"Password must be at least {MinPasswordLength} characters");
        else if (Encoding.UTF8.GetByteCount(password) > MaxPasswordBytes)
            errors.Add($"Password must be at most {MaxPasswordBytes} bytes");

        if (errors.Count > 0)
            throw new RequestValidationException(string.Join("; ", errors));
    }
}
