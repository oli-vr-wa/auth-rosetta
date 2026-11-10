using AuthRosetta.Api.Helpers;
using AuthRosetta.Application.DTOs;
using AuthRosetta.Application.Interfaces;
using AuthRosetta.Infrastructure.Services;
using System.Security.Claims;

namespace AuthRosetta.Api.Endpoints;

public static class AuthEndpoints
{
    /// <summary>
    /// Maps the authentication endpoints to the specified <see cref="IEndpointRouteBuilder"/>.
    /// </summary>
    /// <param name="app">The endpoint route builder to which the authentication endpoints will be mapped.</param>
    /// <returns>The updated endpoint route builder.</returns>
    public static IEndpointRouteBuilder MapAuthEndpoints(this IEndpointRouteBuilder app)
    {
        var group = app.MapGroup("/api/auth");

        group.MapPost("/register", Register).AllowAnonymous();
        group.MapPost("2fa/confirm", ConfirmTwoFactorAuthentication).AllowAnonymous();
        group.MapPost("/login", Login).AllowAnonymous();
        group.MapGet("/me", Me);
        group.MapPost("logout", Logout);

        return app;
    }

    /// <summary>
    /// Registers a new user with the given email and password.
    /// </summary>
    /// <param name="request">The registration request containing email and password.</param>
    /// <param name="authService">The authentication service.</param>
    /// <returns>The result of the registration attempt.</returns>
    private static async Task<IResult> Register(RegisterRequest request, IAuthService authService)
    {
        if (string.IsNullOrWhiteSpace(request.Email) 
            || string.IsNullOrWhiteSpace(request.Password)
            || request.Password.Length < 8)
            return Results.BadRequest(new { error = "Valid email and a password of 8+ characters are required" });

        var result = await authService.RegisterAsync(request.Email, request.Password);

        return result is null 
            ? Results.Conflict(new { error = "Email already registered" })
            : Results.Created("/api/auth/me", new { otpauthUri = result.OtpauthUri, secret = result.Secret });
    }

    /// <summary>
    /// Confirms two-factor authentication for a user with the given email and code.
    /// </summary>
    /// <param name="request">The confirmation request containing email and code.</param>
    /// <param name="authService">The authentication service.</param>
    /// <returns>The result of the confirmation attempt.</returns>
    private static async Task<IResult> ConfirmTwoFactorAuthentication(ConfirmRequest request, IAuthService authService)
    {
        return await authService.ConfirmTwoFactorAuthenticationAsync(request.Email, request.Code)
            ? Results.Ok(new { message = "Two-factor authentication confirmed" })
            : Results.BadRequest(new { error = "Invalid code" });
    }

    /// <summary>
    /// Logs in a user with the given email, password, and two-factor authentication code.
    /// </summary>
    /// <param name="request">The login request containing email, password, and code.</param>
    /// <param name="authService">The authentication service.</param>
    /// <param name="jwt">The JWT options.</param>
    /// <param name="ctx">The HTTP context.</param>
    /// <returns>The result of the login attempt.</returns>
    private static async Task<IResult> Login(LoginRequest request, IAuthService authService, JwtOptions jwt, HttpContext ctx)
    {
        var token = await authService.LoginAsync(request.Email, request.Password, request.Code);

        if (token is null)
            return Results.Json(new { error = "Invalid credentials" }, statusCode: StatusCodes.Status401Unauthorized);

        AuthCookie.Set(ctx, token, jwt.ExpiryMinutes);
        return Results.Ok(new { message = "Login successful" });
    }

    /// <summary>
    /// Gets the authenticated user's email from the claims principal.
    /// </summary>
    /// <param name="user">The claims principal representing the authenticated user.</param>
    /// <returns>The result containing the user's email.</returns>
    private static async Task<IResult> Me(ClaimsPrincipal user) => Results.Ok(new { email = user.FindFirstValue("email") });

    /// <summary>
    /// Logs out the authenticated user by revoking their token.
    /// </summary>
    /// <param name="user">The claims principal representing the authenticated user.</param>
    /// <param name="authService">The authentication service.</param>
    /// <param name="ctx">The HTTP context.</param>
    /// <returns>The result of the logout operation.</returns>
    private static async Task<IResult> Logout(ClaimsPrincipal user, IAuthService authService, HttpContext ctx)
    {
        var jti = user.FindFirstValue("jti");
        var expClaim = user.FindFirstValue("exp");

        if (jti is null || expClaim is null) return Results.Unauthorized();

        var expiresAt = DateTimeOffset.FromUnixTimeSeconds(long.Parse(expClaim)).UtcDateTime;
        await authService.LogoutAsync(jti, expiresAt);

        AuthCookie.Clear(ctx);
        return Results.NoContent();
    }
}
