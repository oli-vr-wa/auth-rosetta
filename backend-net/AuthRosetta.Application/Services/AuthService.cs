using AuthRosetta.Application.DTOs;
using AuthRosetta.Application.Interfaces;
using AuthRosetta.Domain.Entities.Identity;

namespace AuthRosetta.Application.Services;

public class AuthService(
    IUserRepository userRepository,
    IRevokedTokenRepository revokedTokenRepository,
    IPasswordService passwordService,
    ITotpService totpService,
    ITokenService tokenService) : IAuthService
{
    private static string NormalizeEmail(string email) => email.Trim().ToLowerInvariant();

    /// <inheritdoc />
    public async Task<RegisterResult?> RegisterAsync(string email, string password)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(email, nameof(email));
        ArgumentException.ThrowIfNullOrWhiteSpace(password, nameof(password));

        email = NormalizeEmail(email);
        if (await userRepository.GetByEmailAsync(email) is not null) return null;

        var secret = totpService.GenerateSecret();
        userRepository.Add(new User
        {
            Email = email,
            PasswordHash = passwordService.Hash(password),
            TotpSecret = secret
        });

        await userRepository.SaveChangesAsync();

        return new RegisterResult(totpService.BuildUri(email, secret), secret);
    }

    /// <inheritdoc />
    public async Task<bool> ConfirmTwoFactorAuthenticationAsync(string email, string code)
    {
        var user = await userRepository.GetByEmailAsync(NormalizeEmail(email));
        if (user is null || !totpService.Verify(user.TotpSecret, code)) return false;

        user.TwoFactorEnabled = true;
        await userRepository.SaveChangesAsync();
        return true;
    }

    /// <inheritdoc />
    public async Task<string?> LoginAsync(string email, string password, string code)
    {
        var user = await userRepository.GetByEmailAsync(NormalizeEmail(email));
        if (user is null 
            || !passwordService.Verify(password, user.PasswordHash)
            || !user.TwoFactorEnabled
            || !totpService.Verify(user.TotpSecret, code)) 
            return null;

        return tokenService.CreateToken(user.Id, user.Email);
    }    

    /// <inheritdoc />
    public Task LogoutAsync(string jti, DateTime expiresAt) => revokedTokenRepository.RevokeAsync(jti, expiresAt);
}
