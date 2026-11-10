namespace AuthRosetta.Application.Interfaces;

public interface IRevokedTokenRepository
{
    /// <summary>
    /// Revokes a token by its JTI (JWT ID) and sets its expiration time. 
    /// </summary>
    /// <param name="jti">The JTI (JWT ID) of the token to revoke.</param>
    /// <param name="expiresAt">The expiration time of the token.</param>
    /// <returns>A task that represents the asynchronous operation.</returns>
    Task RevokeAsync(string jti, DateTime expiresAt);

    /// <summary>
    /// Checks if a token with the specified JTI (JWT ID) has been revoked.
    /// </summary>
    /// <param name="jti">The JTI (JWT ID) of the token to check.</param>
    /// <returns>A task that represents the asynchronous operation. The task result contains true if the token has been revoked; otherwise, false.</returns>
    Task<bool> IsRevokedAsync(string jti);
}
