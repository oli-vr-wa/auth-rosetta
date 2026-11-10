namespace AuthRosetta.Application.Interfaces;

public interface ITokenService
{
    /// <summary>
    /// Creates a JWT token for the given user ID and email.
    /// </summary>
    /// <param name="userId">The User ID</param>
    /// <param name="email">The User's email</param>
    /// <returns>The created JWT token</returns>
    string CreateToken(Guid userId, string email);
}
