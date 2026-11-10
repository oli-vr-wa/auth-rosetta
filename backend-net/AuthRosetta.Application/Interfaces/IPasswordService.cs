namespace AuthRosetta.Application.Interfaces;

public interface IPasswordService
{
    /// <summary>
    /// Hashes the provided password using a secure hashing algorithm (BCrypt) with a specified work factor.
    /// </summary>
    /// <param name="password">The password to hash.</param>
    /// <returns>The hashed password.</returns>
    string Hash(string password);

    /// <summary>
    /// Verifies a password against a hashed password.
    /// </summary>
    /// <param name="password">The password to verify.</param>
    /// <param name="hash">The hashed password to compare against.</param>
    /// <returns>True if the password matches the hash; otherwise, false.</returns>
    bool Verify(string password, string hash);
}
