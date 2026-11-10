using AuthRosetta.Application.Interfaces;

namespace AuthRosetta.Infrastructure.Services;

public class PasswordService : IPasswordService
{
    /// <inheritdoc />
    public string Hash(string password) => BCrypt.Net.BCrypt.HashPassword(password, workFactor: 12);

    /// <inheritdoc />
    public bool Verify(string password, string hash) => BCrypt.Net.BCrypt.Verify(password, hash);
}
