namespace AuthRosetta.Application.Interfaces;

public interface ITotpService
{
    /// <summary>
    /// Generates a new secret key for TOTP (Time-based One-Time Password) authentication.
    /// </summary>
    /// <returns>The generated secret key.</returns>
    string GenerateSecret();

    /// <summary>
    /// Builds a URI for TOTP authentication based on the provided email and secret key. 
    /// This URI can be used to generate a QR code for easy setup in authenticator apps.
    /// </summary>
    /// <param name="email">The email address of the user.</param>
    /// <param name="secret">The secret key for TOTP authentication.</param>
    /// <returns>The URI for TOTP authentication.</returns>
    string BuildUri(string email, string secret);

    /// <summary>
    /// Verifies a TOTP code against the provided secret key.
    /// </summary>
    /// <param name="secret">The secret key for TOTP authentication.</param>
    /// <param name="code">The TOTP code to verify.</param>
    /// <returns>True if the code is valid; otherwise, false.</returns>
    bool Verify(string secret, string code);
}
