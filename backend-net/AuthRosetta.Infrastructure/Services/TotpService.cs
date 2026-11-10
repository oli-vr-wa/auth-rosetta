using AuthRosetta.Application.Interfaces;
using OtpNet;

namespace AuthRosetta.Infrastructure.Services;

public class TotpService : ITotpService
{
    private const string Issuer = "AuthRosetta";

    /// <inheritdoc />
    public string GenerateSecret() => Base32Encoding.ToString(KeyGeneration.GenerateRandomKey(20));

    /// <inheritdoc />
    public string BuildUri(string email, string secret) =>
        $"otpauth://totp/{Uri.EscapeDataString(Issuer)}:{Uri.EscapeDataString(email)}?secret={secret}&issuer={Uri.EscapeDataString(Issuer)}&digits=6&period=60";

    /// <inheritdoc />
    public bool Verify(string secret, string code) => new Totp(Base32Encoding.ToBytes(secret)).VerifyTotp(code, out _, VerificationWindow.RfcSpecifiedNetworkDelay);
}
