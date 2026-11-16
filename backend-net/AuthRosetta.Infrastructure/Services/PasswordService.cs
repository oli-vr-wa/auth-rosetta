using AuthRosetta.Application.Interfaces;

namespace AuthRosetta.Infrastructure.Services;

public class PasswordService : IPasswordService
{
    private const int WorkFactor = 12; // Adjust the work factor as needed for security and performance trade-offs.

    private static readonly string DummyHash = BCrypt.Net.BCrypt.HashPassword(Guid.NewGuid().ToString(), workFactor: WorkFactor);

    /// <inheritdoc />
    public string Hash(string password) => BCrypt.Net.BCrypt.HashPassword(password, workFactor: WorkFactor);

    /// <inheritdoc />
    public bool Verify(string password, string hash) => BCrypt.Net.BCrypt.Verify(password, hash);

    /// <inheritdoc />
    public bool VerifyAgainstDummy(string password)
    {
        BCrypt.Net.BCrypt.Verify(password, DummyHash);
        return false; // Always return false since this is a dummy verification.
    }
}
