namespace AuthRosetta.Api.Helpers;

/// <summary>
/// Provides helper methods for managing the authentication cookie.
/// </summary>
public static class AuthCookie
{
    public const string Name = "access_token";

    /// <summary>
    /// Returns the cookie options for the authentication cookie based on the provided HTTP context.
    /// </summary>
    /// <param name="ctx">The HTTP context.</param>
    /// <returns>The cookie options.</returns>
    private static CookieOptions Options(HttpContext ctx) => new()
    {
        HttpOnly = true,
        Secure = ctx.Request.IsHttps,
        SameSite = SameSiteMode.Strict,
        Path = "/",
    };

    /// <summary>
    /// Sets the authentication cookie in the response with the specified token and expiration time.
    /// </summary>
    /// <param name="ctx">The HTTP context.</param>
    /// <param name="token">The authentication token.</param>
    /// <param name="minutes">The expiration time in minutes.</param>
    public static void Set(HttpContext ctx, string token, int minutes)
    {
        var options = Options(ctx);
        options.Expires = DateTimeOffset.UtcNow.AddMinutes(minutes);
        ctx.Response.Cookies.Append(Name, token, options);
    }

    /// <summary>
    /// Clears the authentication cookie from the response.
    /// </summary>
    /// <param name="ctx">The HTTP context.</param>
    public static void Clear(HttpContext ctx) => ctx.Response.Cookies.Delete(Name, Options(ctx));
}
