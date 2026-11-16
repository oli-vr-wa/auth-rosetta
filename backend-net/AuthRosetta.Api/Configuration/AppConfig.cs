namespace AuthRosetta.Api.Configuration;

/// <summary>
/// Represents the application configuration loaded from environment variables.
/// </summary>
/// <param name="DbUser">The database user.</param>
/// <param name="DbPassword">The database password.</param>
/// <param name="JwtKey">The JWT key.</param>
/// <param name="FrontendOrigin">The frontend origin.</param>
public sealed record AppConfig(string DbUser, string DbPassword, string JwtKey, string FrontendOrigin)
{
    public const string DefaultFrontendOrigin = "http://localhost:5173";

    /// <summary>
    /// Creates an <see cref="AppConfig"/> instance from environment variables.
    /// </summary>
    /// <returns>Returns an <see cref="AppConfig"/> instance.</returns>
    /// <exception cref="InvalidOperationException">Thrown when required environment variables are missing or invalid.</exception>
    public static AppConfig FromEnvironment()
    {
        static string? Read(string name) => Environment.GetEnvironmentVariable(name);

        var missing = new[] { "DB_USER", "DB_PASSWORD", "JWT_KEY" }
            .Where(name => string.IsNullOrWhiteSpace(Read(name)))
            .ToList();
        if (missing.Count > 0)
            throw new InvalidOperationException($"Missing required environment variables: {string.Join(", ", missing)}");

        var jwtKey = Read("JWT_KEY")!;
        if (jwtKey.Length < 32)
            throw new InvalidOperationException("JWT_KEY must be at least 32 characters long");

        var origin = Read("FRONTEND_ORIGIN")?.Trim();
        if (string.IsNullOrWhiteSpace(origin))
            origin = DefaultFrontendOrigin;
        if (!IsPlainOrigin(origin))
            throw new InvalidOperationException("FRONTEND_ORIGIN must be a plain origin such as http://localhost:5173 (no wildcards, no paths, no trailing slash)");

        return new AppConfig(Read("DB_USER")!, Read("DB_PASSWORD")!, jwtKey, origin);
    }

    /// <summary>
    /// Determines whether the specified value is a plain origin (e.g., http://localhost:5173).
    /// </summary>
    /// <param name="value">The value to check.</param>
    /// <returns>Returns <c>true</c> if the value is a plain origin; otherwise, <c>false</c>.</returns>
    private static bool IsPlainOrigin(string value) =>
        Uri.TryCreate(value, UriKind.Absolute, out var uri)
        && (uri.Scheme == Uri.UriSchemeHttp || uri.Scheme == Uri.UriSchemeHttps)
        && uri.GetLeftPart(UriPartial.Authority) == value;
}
