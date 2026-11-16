using AuthRosetta.Application.DTOs;

namespace AuthRosetta.Application.Interfaces;

public interface IAuthService
{
    /// <summary>
    /// Registers a new user with the given email and password.
    /// </summary>
    /// <param name="request">The registration request containing the email and password of the user to register.</param>
    /// <returns>The result of the registration.</returns>
    Task<RegisterResult?> RegisterAsync(RegisterRequest request);

    /// <summary>
    /// Attempts to log in a user with the given email and password.
    /// </summary>
    /// <param name="request">The confirmation request containing the email and two-factor authentication code of the user to confirm.</param>
    /// <returns>True if the confirmation is successful; otherwise, false.</returns>
    Task<bool> ConfirmTwoFactorAuthenticationAsync(ConfirmRequest request);

    /// <summary>
    /// Attempts to log in a user with the given email, password, and two-factor authentication code.
    /// </summary>
    /// <param name="request">The login request containing the email, password, and two-factor authentication code of the user to log in.</param>
    /// <returns>The authentication token if the login is successful; otherwise, null.</returns>
    Task<string?> LoginAsync(LoginRequest request);

    /// <summary>
    /// Logs out a user by invalidating their authentication token.
    /// </summary>
    /// <param name="jti">The unique identifier of the token to invalidate.</param>
    /// <param name="expiresAt">The expiration date and time of the token.</param>
    /// <returns>A task that represents the asynchronous operation.</returns>
    Task LogoutAsync(string jti, DateTime expiresAt);
}
